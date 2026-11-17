package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MoeForge/moego/ojson"
	"github.com/cliproxyapi-plugins/model-quality/login/account"
	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
)

type options struct{ Source, AuthDir, Destination, QualityState, QualityDB string }
type report struct {
	QualityChecked           bool   `json:"quality_checked"`
	Accounts                 int    `json:"accounts"`
	AccountBindingsConverted int    `json:"account_bindings_converted"`
	Jobs                     int    `json:"jobs"`
	JobBindingsConverted     int    `json:"job_bindings_converted"`
	VaultEntries             int    `json:"vault_entries"`
	VaultBindingsConverted   int    `json:"vault_bindings_converted"`
	Buckets                  int    `json:"buckets"`
	Records                  int    `json:"records"`
	SourceSHA256             string `json:"source_sha256"`
	DestinationSHA256        string `json:"destination_sha256"`
}

// These are validation projections only. Records are never re-encoded from them.
type credentials struct{ Email, Password, TOTPSecret string }
type storedAccount struct {
	View                 struct{ ID, AuthID, IdentityVersion string }
	EmailHash, Principal string
	SecretRevision       uint64
	Credentials          credentials
}
type storedJob struct {
	AdoptedIdentity, AdoptedPrincipal string
	View                              struct{ ID, Status, TargetAuthID, CandidateID string }
}
type vaultEntry struct {
	Revision            uint64
	Identity, Principal string
	Credentials         credentials
}
type binding struct{ Old, New, Principal, Email string }
type replacement struct {
	Bucket, Key string
	Value       []byte
}

func digest(values ...string) string {
	h := sha256.New()
	for _, value := range values {
		h.Write([]byte(value))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
func emailHash(key []byte, email string) string {
	h := hmac.New(sha256.New, key)
	for _, value := range []string{"account-email", email} {
		h.Write([]byte(value))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", errors.New("file_read_failed")
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", errors.New("file_read_failed")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func noSymlinks(path string) error {
	for current := path; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe_path")
		}
		if filepath.Dir(current) == current {
			return nil
		}
	}
}
func safeAuthID(id string) bool {
	return id != "" && id != "." && filepath.Base(id) == id && !strings.ContainsAny(id, "/\\:\x00") && strings.HasSuffix(id, ".json")
}
func principalFor(authDir, id string) (account.LoginPrincipal, error) {
	if !safeAuthID(id) {
		return account.LoginPrincipal{}, errors.New("unsafe_auth_binding")
	}
	path := filepath.Join(authDir, id)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 8<<20 {
		return account.LoginPrincipal{}, errors.New("credential_unavailable")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return account.LoginPrincipal{}, errors.New("credential_unavailable")
	}
	// Project only identity strings. The original credential JSON is never rewritten.
	if err = uniqueJSON(raw); err != nil {
		return account.LoginPrincipal{}, err
	}
	root := ojson.ParseBytes(raw)
	metadata := map[string]any{}
	if !root.IsObject() {
		return account.LoginPrincipal{}, errors.New("credential_invalid")
	}
	for _, key := range []string{"type", "id_token", "access_token", "account_id", "email"} {
		value := root.Get(key)
		if value.Exists() && value.Type != ojson.String {
			return account.LoginPrincipal{}, errors.New("credential_invalid")
		}
		if value.Exists() {
			metadata[key] = value.String()
		}
	}
	if metadata["type"] != "codex" {
		return account.LoginPrincipal{}, errors.New("credential_provider_mismatch")
	}
	p, err := account.LoginPrincipalFromAuth(&account.Auth{ID: id, Provider: "codex", Metadata: metadata})
	if err != nil || p.Email == "" {
		return account.LoginPrincipal{}, errors.New("credential_identity_invalid")
	}
	return p, nil
}
func uniqueJSON(raw []byte) error {
	if !ojson.ValidBytes(raw) {
		return errors.New("credential_invalid")
	}
	var visit func(ojson.Result) error
	visit = func(value ojson.Result) error {
		if value.IsObject() {
			keys := map[string]bool{}
			var failure error
			value.ForEach(func(key, child ojson.Result) bool {
				name := key.String()
				if keys[name] {
					failure = errors.New("credential_invalid")
					return false
				}
				keys[name] = true
				failure = visit(child)
				return failure == nil
			})
			return failure
		}
		if value.IsArray() {
			for _, child := range value.Array() {
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(ojson.ParseBytes(raw))
}
func verifiedBinding(authDir, id, old, principal, email string, known map[string]binding) (binding, error) {
	parsed, err := uuid.Parse(old)
	if err != nil || old != parsed.String() || parsed == uuid.Nil {
		return binding{}, errors.New("legacy_identity_invalid")
	}
	p, err := principalFor(authDir, id)
	if err != nil {
		return binding{}, err
	}
	if principal == "" || p.Key() != principal || email == "" || strings.ToLower(strings.TrimSpace(email)) != p.Email {
		return binding{}, errors.New("binding_identity_conflict")
	}
	b := binding{Old: old, New: digest("identity", id, principal), Principal: principal, Email: p.Email}
	if previous, ok := known[id]; ok && previous != b {
		return binding{}, errors.New("binding_version_conflict")
	}
	known[id] = b
	return b, nil
}
func collect(tx *bolt.Tx, authDir string, r *report) ([]replacement, error) {
	for _, name := range []string{"meta", "accounts", "jobs", "vault", "idempotency"} {
		if tx.Bucket([]byte(name)) == nil {
			return nil, errors.New("source_schema_invalid")
		}
	}
	meta := tx.Bucket([]byte("meta"))
	key := meta.Get([]byte("hmac"))
	if string(meta.Get([]byte("schema"))) != "2" || len(key) != 32 {
		return nil, errors.New("source_schema_invalid")
	}
	replacements := []replacement{}
	bindings := map[string]binding{}
	err := tx.Bucket([]byte("accounts")).ForEach(func(k, raw []byte) error {
		var a storedAccount
		if err := decodeGob(raw, &a); err != nil {
			return err
		}
		r.Accounts++
		if a.View.ID != string(k) || a.SecretRevision == 0 || a.Credentials.Email == "" || a.Credentials.Password == "" || a.Credentials.TOTPSecret == "" || a.EmailHash != emailHash(key, a.Credentials.Email) {
			return errors.New("account_record_invalid")
		}
		if a.View.AuthID == "" {
			if a.View.IdentityVersion != "" || a.Principal != "" {
				return errors.New("unbound_account_conflict")
			}
			return nil
		}
		b, err := verifiedBinding(authDir, a.View.AuthID, a.View.IdentityVersion, a.Principal, a.Credentials.Email, bindings)
		if err != nil {
			return err
		}
		next, err := replaceGobString(raw, b.Old, b.New)
		if err != nil {
			return err
		}
		var check storedAccount
		if err = decodeGob(next, &check); err != nil || check.View.IdentityVersion != b.New {
			return errors.New("account_patch_invalid")
		}
		replacements = append(replacements, replacement{"accounts", string(k), next})
		r.AccountBindingsConverted++
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = tx.Bucket([]byte("vault")).ForEach(func(k, raw []byte) error {
		var v vaultEntry
		if err := decodeGob(raw, &v); err != nil {
			return err
		}
		r.VaultEntries++
		if v.Revision == 0 || v.Credentials.Password == "" || v.Credentials.TOTPSecret == "" {
			return errors.New("vault_record_invalid")
		}
		b, err := verifiedBinding(authDir, string(k), v.Identity, v.Principal, v.Credentials.Email, bindings)
		if err != nil {
			return err
		}
		next, err := replaceGobString(raw, b.Old, b.New)
		if err != nil {
			return err
		}
		var check vaultEntry
		if err = decodeGob(next, &check); err != nil || check.Identity != b.New {
			return errors.New("vault_patch_invalid")
		}
		replacements = append(replacements, replacement{"vault", string(k), next})
		r.VaultBindingsConverted++
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = tx.Bucket([]byte("jobs")).ForEach(func(k, raw []byte) error {
		var j storedJob
		if err := decodeGob(raw, &j); err != nil {
			return err
		}
		r.Jobs++
		if j.View.ID != string(k) || j.View.CandidateID != "" {
			return errors.New("job_record_invalid")
		}
		switch j.View.Status {
		case "committed", "failed", "cancelled", "discarded", "expired", "interrupted", "interaction_required", "conflict":
		default:
			return errors.New("unresolved_login_job")
		}
		b, ok := bindings[j.View.TargetAuthID]
		if j.View.Status != "committed" || !ok || j.AdoptedIdentity != b.Old || j.AdoptedPrincipal != b.Principal {
			return nil
		}
		next, err := replaceGobString(raw, b.Old, b.New)
		if err != nil {
			return err
		}
		var check storedJob
		if err = decodeGob(next, &check); err != nil || check.AdoptedIdentity != b.New {
			return errors.New("job_patch_invalid")
		}
		replacements = append(replacements, replacement{"jobs", string(k), next})
		r.JobBindingsConverted++
		return nil
	})
	return replacements, err
}

func upgrade(o options) (r report, returned error) {
	if (o.QualityState == "") != (o.QualityDB == "") {
		return r, errors.New("quality_paths_must_be_paired")
	}
	source, err := filepath.Abs(o.Source)
	if err != nil {
		return r, errors.New("invalid_path")
	}
	authDir, err := filepath.Abs(o.AuthDir)
	if err != nil {
		return r, errors.New("invalid_path")
	}
	destination, err := filepath.Abs(o.Destination)
	if err != nil || source == destination {
		return r, errors.New("invalid_path")
	}
	if _, err = os.Lstat(destination); err == nil || !errors.Is(err, os.ErrNotExist) {
		return r, errors.New("destination_exists")
	}
	if err = noSymlinks(source); err != nil {
		return r, err
	}
	if err = noSymlinks(authDir); err != nil {
		return r, err
	}
	info, err := os.Stat(source)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 256<<20 {
		return r, errors.New("source_file_invalid")
	}
	info, err = os.Stat(authDir)
	if err != nil || !info.IsDir() {
		return r, errors.New("auth_directory_invalid")
	}
	db, err := bolt.Open(source, 0600, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if err != nil {
		return r, errors.New("source_open_failed")
	}
	defer db.Close()
	r.SourceSHA256, err = fileHash(source)
	if err != nil {
		return r, err
	}
	if o.QualityState != "" {
		closeQuality, checkErr := checkQuality(o.QualityState, o.QualityDB)
		if checkErr != nil {
			return r, checkErr
		}
		defer closeQuality()
		r.QualityChecked = true
	}
	parent := filepath.Dir(destination)
	if err = os.MkdirAll(parent, 0700); err != nil {
		return r, errors.New("destination_directory_invalid")
	}
	if err = noSymlinks(parent); err != nil {
		return r, err
	}
	info, err = os.Stat(parent)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		return r, errors.New("destination_directory_invalid")
	}
	temp, err := os.CreateTemp(parent, ".state-upgrade-")
	if err != nil {
		return r, errors.New("destination_create_failed")
	}
	tempPath := temp.Name()
	temp.Close()
	defer os.Remove(tempPath)
	var changes []replacement
	err = db.View(func(tx *bolt.Tx) error {
		var err error
		changes, err = collect(tx, authDir, &r)
		if err != nil {
			return err
		}
		return tx.CopyFile(tempPath, 0600)
	})
	if err != nil {
		return r, errors.New("source_validation_failed:" + safeError(err))
	}
	target, err := bolt.Open(tempPath, 0600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return r, errors.New("destination_open_failed")
	}
	err = target.Update(func(tx *bolt.Tx) error {
		for _, change := range changes {
			if err := tx.Bucket([]byte(change.Bucket)).Put([]byte(change.Key), change.Value); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		err = target.View(func(tx *bolt.Tx) error {
			for e := range tx.Check() {
				if e != nil {
					return e
				}
			}
			return nil
		})
	}
	if err == nil {
		err = comparePreserved(db, target, changes, &r)
	}
	if err == nil {
		err = target.Sync()
	}
	closeErr := target.Close()
	if err != nil || closeErr != nil {
		return r, errors.New("destination_verification_failed")
	}
	after, err := fileHash(source)
	if err != nil || after != r.SourceSHA256 {
		return r, errors.New("source_changed")
	}
	r.DestinationSHA256, err = fileHash(tempPath)
	if err != nil {
		return r, err
	}
	if err = os.Link(tempPath, destination); err != nil {
		return r, errors.New("destination_publish_failed")
	}
	directory, err := os.Open(parent)
	if err != nil {
		_ = os.Remove(destination)
		return r, errors.New("destination_sync_failed")
	}
	syncErr := directory.Sync()
	directory.Close()
	if syncErr != nil {
		_ = os.Remove(destination)
		return r, errors.New("destination_sync_failed")
	}
	return r, nil
}
func safeError(err error) string {
	// Only fixed validation labels originating in this file are safe to report.
	value := err.Error()
	for _, ch := range value {
		if ch < 'a' || ch > 'z' {
			if ch != '_' {
				return "database_copy_failed"
			}
		}
	}
	return value
}
func comparePreserved(source, target *bolt.DB, changes []replacement, r *report) error {
	modified := map[string]map[string][]byte{}
	for _, c := range changes {
		if modified[c.Bucket] == nil {
			modified[c.Bucket] = map[string][]byte{}
		}
		modified[c.Bucket][c.Key] = c.Value
	}
	return source.View(func(old *bolt.Tx) error {
		return target.View(func(next *bolt.Tx) error {
			var compare func(*bolt.Bucket, *bolt.Bucket, string) error
			compare = func(a, b *bolt.Bucket, top string) error {
				if b == nil || a.Sequence() != b.Sequence() {
					return errors.New("bucket_changed")
				}
				r.Buckets++
				count := 0
				if err := a.ForEach(func(k, v []byte) error {
					count++
					if v == nil {
						return compare(a.Bucket(k), b.Bucket(k), "")
					}
					r.Records++
					want := v
					if replacement := modified[top][string(k)]; replacement != nil {
						want = replacement
					}
					if !bytes.Equal(want, b.Get(k)) {
						return errors.New("record_changed")
					}
					return nil
				}); err != nil {
					return err
				}
				other := 0
				_ = b.ForEach(func([]byte, []byte) error { other++; return nil })
				if count != other {
					return errors.New("record_count_changed")
				}
				return nil
			}
			count := 0
			if err := old.ForEach(func(k []byte, b *bolt.Bucket) error { count++; return compare(b, next.Bucket(k), string(k)) }); err != nil {
				return err
			}
			other := 0
			_ = next.ForEach(func([]byte, *bolt.Bucket) error { other++; return nil })
			if count != other {
				return errors.New("bucket_count_changed")
			}
			return nil
		})
	})
}
