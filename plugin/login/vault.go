package login

import (
	"bytes"
	"encoding/gob"
	"errors"
	"sync"

	auth "github.com/cliproxyapi-plugins/model-quality/login/account"
	bolt "go.etcd.io/bbolt"
)

type vault struct {
	mu      sync.Mutex
	journal *journal
	manager *auth.Manager
}
type vaultEntry struct {
	Revision                         uint64
	Identity, Principal, MaskedEmail string
	Credentials                      Credentials
}

func storedCredentialsComplete(c Credentials) bool {
	return c.Email != "" && c.Password != "" && c.TOTPSecret != ""
}

// Plaintext storage is ready once the schema-2 journal has opened successfully.
func (v *vault) ready() error {
	if err := v.journal.db.View(func(tx *bolt.Tx) error {
		if string(tx.Bucket([]byte("meta")).Get([]byte("schema"))) != "2" {
			return errors.New("vault_unavailable")
		}
		return nil
	}); err != nil {
		return errors.New("vault_unavailable")
	}
	return nil
}
func (v *vault) entry(id string) (vaultEntry, bool, error) {
	var e vaultEntry
	found := false
	err := v.journal.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte("vault")).Get([]byte(id))
		if raw == nil {
			return nil
		}
		found = true
		return gob.NewDecoder(bytes.NewReader(raw)).Decode(&e)
	})
	if err != nil {
		return e, false, errors.New("vault_corrupt")
	}
	return e, found, nil
}
func (v *vault) status(id string) map[string]any {
	v.mu.Lock()
	defer v.mu.Unlock()
	err := v.ready()
	out := map[string]any{"available": err == nil, "initialized": err == nil, "storage_mode": "plaintext"}
	if err != nil {
		out["reason"] = SafeCode(err)
	}
	if id != "" {
		e, found, eerr := v.entry(id)
		if eerr != nil {
			out["available"] = false
			out["reason"] = SafeCode(eerr)
		} else if found {
			snapshot, snapshotErr := v.manager.LoginSnapshot(id)
			matches := snapshotErr == nil && snapshot.IdentityVersion == e.Identity && snapshot.Principal == e.Principal
			entry := map[string]any{"revision": e.Revision, "masked_email": e.MaskedEmail, "has_password": matches, "has_totp": matches}
			out["entry"] = entry
			if credentials, readErr := v.credentials(e); readErr == nil {
				entry["email"] = credentials.Email
			} else {
				entry["has_password"], entry["has_totp"] = false, false
				out["reason"] = SafeCode(readErr)
			}
			if !matches {
				out["reason"] = "vault_identity_changed"
			}
		}
	}
	return out
}
func (v *vault) save(id string, revision uint64, c Credentials, afterCommit bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	snapshot, err := v.manager.LoginSnapshot(id)
	if err != nil {
		return errors.New("vault_identity_changed")
	}
	a, ok := v.manager.GetByID(id)
	if !ok {
		return errors.New("vault_identity_changed")
	}
	p, err := auth.LoginPrincipalFromAuth(a)
	if err != nil || p.Email != c.Email {
		return errors.New("vault_identity_changed")
	}
	old, found, err := v.entry(id)
	if err != nil {
		return err
	}
	if !afterCommit && ((found && old.Revision != revision) || (!found && revision != 0)) {
		return errors.New("vault_conflict")
	}
	e := vaultEntry{Revision: old.Revision + 1, Identity: snapshot.IdentityVersion, Principal: snapshot.Principal, MaskedEmail: Mask(c.Email), Credentials: c}
	if err = v.journal.db.Update(func(tx *bolt.Tx) error { return putGob(tx, "vault", id, e) }); err != nil {
		return errors.New("vault_unavailable")
	}
	return nil
}
func (v *vault) read(id string) (Credentials, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	var c Credentials
	e, found, err := v.entry(id)
	if err != nil {
		return c, err
	}
	if !found {
		return c, errors.New("not_found")
	}
	snapshot, err := v.manager.LoginSnapshot(id)
	if err != nil || snapshot.IdentityVersion != e.Identity || snapshot.Principal != e.Principal {
		return c, errors.New("vault_identity_changed")
	}
	return v.credentials(e)
}
func (v *vault) credentials(e vaultEntry) (Credentials, error) {
	if e.Revision == 0 || e.Identity == "" || e.Principal == "" || !storedCredentialsComplete(e.Credentials) {
		return Credentials{}, errors.New("vault_corrupt")
	}
	return e.Credentials, nil
}
func (v *vault) delete(id string, revision uint64) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	old, found, err := v.entry(id)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if old.Revision != revision {
		return errors.New("vault_conflict")
	}
	if err = v.journal.db.Update(func(tx *bolt.Tx) error { return tx.Bucket([]byte("vault")).Delete([]byte(id)) }); err != nil {
		return errors.New("vault_unavailable")
	}
	return nil
}
func (s *Service) VaultStatus(id string) any { return s.vault.status(id) }
func (s *Service) PutVault(id string, revision uint64, c Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.fault {
		return errors.New("storage_unavailable")
	}
	for _, j := range s.jobs {
		if securityBlocks(j.record) && j.record.RateKey == s.journal.fingerprint(c.Email) {
			return errors.New("security_pending")
		}
	}
	if err := s.vaultMutationAllowedLocked(id); err != nil {
		return err
	}
	if err := s.journal.admit(); err != nil {
		return err
	}
	return s.vault.save(id, revision, c, false)
}
func (s *Service) DeleteVault(id string, revision uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.fault {
		return errors.New("storage_unavailable")
	}
	if err := s.vaultMutationAllowedLocked(id); err != nil {
		return err
	}
	return s.vault.delete(id, revision)
}
func (s *Service) vaultMutationAllowedLocked(id string) error {
	for _, j := range s.jobs {
		if j.record.View.TargetAuthID == id && (!terminal(j.record.View.Status) || j.record.View.Status == "saved_not_adopted" || j.record.View.Status == "commit_unknown" || securityBlocks(j.record)) {
			return errors.New("vault_conflict")
		}
	}
	return nil
}
