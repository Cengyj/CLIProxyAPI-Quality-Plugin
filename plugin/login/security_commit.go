package login

import (
	"bytes"
	"encoding/gob"
	"errors"

	bolt "go.etcd.io/bbolt"
)

// commitSecuritySeed is called while the helper is blocked on its private IPC
// acknowledgement. It does not resolve the operation or release its account lock.
func (s *Service) commitSecuritySeed(j *session) error {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.fault || j.ctx.Err() != nil || s.jobs[j.record.View.ID] != j || j.record.View.Status != "running" || !j.owner.Valid() || j.record.SecurityResolved {
		return errors.New("cancelled")
	}
	a, ok := s.accounts[j.record.View.AccountID]
	if !ok || a.View.LastJobID != j.record.View.ID {
		return errors.New("conflict")
	}
	evidence, err := s.readSecurityJournal(j.record, a.Credentials.Email)
	if err != nil || !evidence.activated() || evidence.uncertain() || evidence.Logout != "not_requested" {
		return errors.New("security_uncertain")
	}
	credentials := a.Credentials
	credentials.TOTPSecret, credentials.TOTPFactorID = evidence.NewSecret, evidence.NewFactor
	if a.Credentials != credentials {
		s.setAccountCredentials(&a, credentials)
		a.View.Revision++
	}
	record := j.record
	record.SecurityAccountID = evidence.AccountID
	record.View.TOTPReset = true
	record.View.SecurityStatus = "totp_saved"
	record.View.Revision++
	a.View.LastJobTOTPReset = true
	a.View.LastJobSecurityStatus = "totp_saved"
	// The account stays quarantined until the running operation exits.
	a.View.SecurityPending = true
	if err := s.persistCredentialBinding(a, record, "", true); err != nil {
		return err
	}
	s.accounts[a.View.ID], j.record = a, record
	if j.credentials.Email != "" {
		j.credentials = credentials
	}
	return nil
}

func (s *Service) bindVerifiedFactor(j *session, original Credentials, id string) error {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.fault || j.ctx.Err() != nil || j.record.View.Status != "running" || !j.owner.Valid() || !validSecurityBinding(id) || len(id) > 512 {
		return errors.New("cancelled")
	}
	expected := original
	if j.record.View.Action == "reset_totp" && j.record.View.TOTPReset {
		evidence, err := s.readSecurityJournal(j.record, original.Email)
		if err != nil || !evidence.activated() {
			return errors.New("security_uncertain")
		}
		expected.TOTPSecret, expected.TOTPFactorID = evidence.NewSecret, evidence.NewFactor
	}
	if expected.TOTPFactorID != "" && expected.TOTPFactorID != id {
		return errors.New("identity_conflict")
	}
	a := s.accounts[j.record.View.AccountID]
	if j.record.View.AccountID != "" {
		if a.View.ID == "" || a.View.LastJobID != j.record.View.ID || a.Credentials.Email != expected.Email || a.Credentials.TOTPSecret != expected.TOTPSecret {
			return errors.New("conflict")
		}
		if a.Credentials.TOTPFactorID != "" && a.Credentials.TOTPFactorID != id {
			return errors.New("identity_conflict")
		}
		if a.Credentials.TOTPFactorID != id {
			a.Credentials.TOTPFactorID = id
			a.View.Revision++
		}
	}
	expected.TOTPFactorID = id
	// A manual login without an account stores the binding with its credential
	// only when the existing explicit remember/commit policy permits that save.
	if a.View.ID != "" {
		if err := s.persistCredentialBinding(a, j.record, expected.TOTPSecret, false); err != nil {
			return err
		}
		s.accounts[a.View.ID] = a
	}
	if j.credentials.Email == expected.Email && j.credentials.TOTPSecret == expected.TOTPSecret {
		j.credentials.TOTPFactorID = id
	}
	return nil
}

// Account, matching vault copies and job evidence share one durable transaction.
// A separate read validates the exact data acknowledged to the child process.
func (s *Service) persistCredentialBinding(a storedAccount, record storedJob, matchingSeed string, reset bool) error {
	if err := s.journal.capacity(true); err != nil {
		s.fault = true
		s.cancel()
		return err
	}
	s.vault.mu.Lock()
	defer s.vault.mu.Unlock()
	changed := map[string]vaultEntry{}
	err := s.journal.db.Update(func(tx *bolt.Tx) error {
		if err := putGob(tx, "accounts", a.View.ID, a); err != nil {
			return err
		}
		if err := s.journal.saveTx(tx, record); err != nil {
			return err
		}
		if err := tx.Bucket([]byte("vault")).ForEach(func(key, raw []byte) error {
			var entry vaultEntry
			if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&entry); err != nil {
				return err
			}
			if entry.Credentials.Email != a.Credentials.Email || !reset && entry.Credentials.TOTPSecret != matchingSeed {
				return nil
			}
			if entry.Credentials.TOTPSecret == a.Credentials.TOTPSecret && entry.Credentials.TOTPFactorID == a.Credentials.TOTPFactorID {
				return nil
			}
			entry.Credentials.TOTPSecret, entry.Credentials.TOTPFactorID = a.Credentials.TOTPSecret, a.Credentials.TOTPFactorID
			entry.Revision++
			changed[string(key)] = entry
			return nil
		}); err != nil {
			return err
		}
		// Mutating a bucket during ForEach invalidates bbolt cursor guarantees.
		for key, entry := range changed {
			if err := putGob(tx, "vault", key, entry); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		err = s.journal.db.View(func(tx *bolt.Tx) error {
			var saved storedAccount
			var job storedJob
			if err := gob.NewDecoder(bytes.NewReader(tx.Bucket([]byte("accounts")).Get([]byte(a.View.ID)))).Decode(&saved); err != nil {
				return err
			}
			if err := gob.NewDecoder(bytes.NewReader(tx.Bucket([]byte("jobs")).Get([]byte(record.View.ID)))).Decode(&job); err != nil {
				return err
			}
			if saved.Credentials != a.Credentials || saved.View.LastJobID != a.View.LastJobID || job.View.ID != record.View.ID || job.View.TOTPReset != record.View.TOTPReset || job.SecurityAccountID != record.SecurityAccountID {
				return errors.New("credential_readback_failed")
			}
			for key, expected := range changed {
				var saved vaultEntry
				if err := gob.NewDecoder(bytes.NewReader(tx.Bucket([]byte("vault")).Get([]byte(key)))).Decode(&saved); err != nil {
					return err
				}
				if saved.Credentials != expected.Credentials || saved.Revision != expected.Revision {
					return errors.New("credential_readback_failed")
				}
			}
			return nil
		})
	}
	if err != nil {
		s.fault = true
		s.cancel()
		return errors.New("storage_unavailable")
	}
	return nil
}
