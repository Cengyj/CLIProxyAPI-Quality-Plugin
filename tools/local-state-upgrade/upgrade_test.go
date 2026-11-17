package main

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MoeForge/moego/base64"
	"github.com/MoeForge/moego/ojson"
	"github.com/cliproxyapi-plugins/model-quality/login/account"
	bolt "go.etcd.io/bbolt"
)

const oldIdentity = "11111111-2222-4333-8444-555555555555"
const otherIdentity = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
const fixtureAuth = "offline-local.json"

type accountFull struct {
	View struct {
		ID, AuthID, IdentityVersion, Label, ProxyURL, HealthReason string
		Revision                                                   uint64
		AutoRelogin                                                bool
	}
	EmailHash, Principal       string
	SecretRevision             uint64
	Credentials                credentials
	Evidence, ConsumedEvidence string
	NextAttempt                time.Time
	FutureFields               map[string]string
}
type jobFull struct {
	AdoptedIdentity, AdoptedPrincipal string
	View                              struct {
		ID, Status, TargetAuthID, CandidateID, ErrorCode string
		Revision                                         uint64
	}
	Created, Updated         time.Time
	AccountEvidence, RateKey string
	AccountSequence          uint64
	LogEntries               []struct {
		ID             uint64
		Level, Message string
	}
	FutureFields []string
}
type vaultFull struct {
	Revision                         uint64
	Identity, Principal, MaskedEmail string
	Credentials                      credentials
	FutureField                      string
}

type fixture struct {
	options         options
	account         accountFull
	job, historical jobFull
	vault           vaultFull
	principal       string
}

func gobBytes(t *testing.T, v any) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(v); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
func makeFixture(t *testing.T) fixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(base, "old")
	authDir := filepath.Join(base, "auths")
	destinationDir := filepath.Join(base, "new")
	for _, p := range []string{sourceDir, authDir, destinationDir} {
		if err = os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	f := fixture{options: options{Source: filepath.Join(sourceDir, "credential-login.db"), AuthDir: authDir, Destination: filepath.Join(destinationDir, "credential-login.db")}}
	claims := `{"sub":"offline-user","email":"offline@example.test","https://api.openai.com/auth":{"chatgpt_account_id":"offline-account","chatgpt_user_id":"offline-user"}}`
	token := "e30." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".offline"
	metadata := map[string]any{"type": "codex", "id_token": token, "access_token": token, "refresh_token": "OFFLINE_REFRESH", "disabled": true}
	document := ojson.NewDocString(`{"custom_policy":{"weight":9007199254740999}}`)
	for key, value := range metadata {
		if err := document.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	authRaw := document.Bytes()
	if err = os.WriteFile(filepath.Join(authDir, fixtureAuth), authRaw, 0600); err != nil {
		t.Fatal(err)
	}
	principal, err := account.LoginPrincipalFromAuth(&account.Auth{Provider: "codex", Metadata: metadata})
	if err != nil {
		t.Fatal(err)
	}
	f.principal = principal.Key()
	f.account.View.ID = "offline-record"
	f.account.View.AuthID = fixtureAuth
	f.account.View.IdentityVersion = oldIdentity
	f.account.View.Label = "fixture label"
	f.account.View.ProxyURL = "socks5://fixture:offline@127.0.0.1:1080"
	f.account.View.Revision = 19
	f.account.View.AutoRelogin = true
	f.account.View.HealthReason = "invalid_grant"
	key := bytes.Repeat([]byte{42}, 32)
	f.account.Credentials = credentials{"offline@example.test", " OFFLINE_PASSWORD ", "OFFLINE_TOTP"}
	f.account.EmailHash = emailHash(key, f.account.Credentials.Email)
	f.account.Principal = f.principal
	f.account.SecretRevision = 7
	f.account.Evidence = "evidence"
	f.account.ConsumedEvidence = "consumed"
	f.account.NextAttempt = time.Unix(123456, 0).UTC()
	f.account.FutureFields = map[string]string{"unknown": "preserved opaque field"}
	f.job.View.ID = "offline-job"
	f.job.View.Status = "committed"
	f.job.View.TargetAuthID = fixtureAuth
	f.job.View.Revision = 29
	f.job.AdoptedIdentity = oldIdentity
	f.job.AdoptedPrincipal = f.principal
	f.job.Created = time.Unix(1234, 567).UTC()
	f.job.Updated = time.Unix(2345, 678).UTC()
	f.job.AccountEvidence = "original evidence"
	f.job.RateKey = "rate hash"
	f.job.AccountSequence = 15
	f.job.FutureFields = []string{"preserve", "unknown"}
	f.job.LogEntries = append(f.job.LogEntries, struct {
		ID             uint64
		Level, Message string
	}{1, "info", "fixture"})
	f.historical = f.job
	f.historical.View.ID = "historical-job"
	f.historical.AdoptedIdentity = otherIdentity
	f.vault = vaultFull{Revision: 4, Identity: oldIdentity, Principal: f.principal, MaskedEmail: "o***@example.test", Credentials: f.account.Credentials, FutureField: "also preserved"}
	db, err := bolt.Open(f.options.Source, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{"meta", "accounts", "jobs", "vault", "idempotency", "unknown_bucket"} {
			if _, err := tx.CreateBucket([]byte(name)); err != nil {
				return err
			}
		}
		meta := tx.Bucket([]byte("meta"))
		meta.Put([]byte("schema"), []byte("2"))
		meta.Put([]byte("hmac"), key)
		meta.Put([]byte("account-settings"), []byte("unchanged opaque settings"))
		meta.SetSequence(41)
		tx.Bucket([]byte("accounts")).Put([]byte(f.account.View.ID), gobBytes(t, f.account))
		tx.Bucket([]byte("jobs")).Put([]byte(f.job.View.ID), gobBytes(t, f.job))
		tx.Bucket([]byte("jobs")).Put([]byte(f.historical.View.ID), gobBytes(t, f.historical))
		tx.Bucket([]byte("vault")).Put([]byte(fixtureAuth), gobBytes(t, f.vault))
		tx.Bucket([]byte("idempotency")).Put([]byte("fixture-key"), []byte("opaque original response"))
		nested, _ := tx.Bucket([]byte("unknown_bucket")).CreateBucket([]byte("nested"))
		nested.SetSequence(7)
		return nested.Put([]byte("opaque"), []byte{0, 1, 255})
	})
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	return f
}
func mutateSource(t *testing.T, f fixture, fn func(*bolt.Tx) error) {
	t.Helper()
	db, err := bolt.Open(f.options.Source, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(fn)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpgradePreservesEveryOtherFieldAndSource(t *testing.T) {
	f := makeFixture(t)
	before, _ := fileHash(f.options.Source)
	authBefore, _ := fileHash(filepath.Join(f.options.AuthDir, fixtureAuth))
	r, err := upgrade(f.options)
	if err != nil {
		t.Fatal(err)
	}
	if r.Accounts != 1 || r.AccountBindingsConverted != 1 || r.Jobs != 2 || r.JobBindingsConverted != 1 || r.VaultBindingsConverted != 1 || r.SourceSHA256 != before {
		t.Fatal("unexpected conversion counts")
	}
	after, _ := fileHash(f.options.Source)
	authAfter, _ := fileHash(filepath.Join(f.options.AuthDir, fixtureAuth))
	if before != after || authBefore != authAfter {
		t.Fatal("source or credential file changed")
	}
	info, _ := os.Stat(f.options.Destination)
	if info.Mode().Perm() != 0600 {
		t.Fatal("destination permission changed")
	}
	target, err := bolt.Open(f.options.Destination, 0600, &bolt.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	err = target.View(func(tx *bolt.Tx) error {
		expected := digest("identity", fixtureAuth, f.principal)
		var a accountFull
		if err := decodeGob(tx.Bucket([]byte("accounts")).Get([]byte(f.account.View.ID)), &a); err != nil {
			return err
		}
		want := f.account
		want.View.IdentityVersion = expected
		if !reflect.DeepEqual(a, want) {
			t.Error("account field changed outside IdentityVersion")
		}
		var j jobFull
		if err := decodeGob(tx.Bucket([]byte("jobs")).Get([]byte(f.job.View.ID)), &j); err != nil {
			return err
		}
		wantJob := f.job
		wantJob.AdoptedIdentity = expected
		if !reflect.DeepEqual(j, wantJob) {
			t.Error("job field changed outside AdoptedIdentity")
		}
		var v vaultFull
		if err := decodeGob(tx.Bucket([]byte("vault")).Get([]byte(fixtureAuth)), &v); err != nil {
			return err
		}
		wantVault := f.vault
		wantVault.Identity = expected
		if !reflect.DeepEqual(v, wantVault) {
			t.Error("vault field changed outside Identity")
		}
		var historical jobFull
		if err := decodeGob(tx.Bucket([]byte("jobs")).Get([]byte(f.historical.View.ID)), &historical); err != nil {
			return err
		}
		if !reflect.DeepEqual(historical, f.historical) {
			t.Error("mismatched historical job changed")
		}
		if string(tx.Bucket([]byte("meta")).Get([]byte("account-settings"))) != "unchanged opaque settings" || tx.Bucket([]byte("meta")).Sequence() != 41 || !bytes.Equal(tx.Bucket([]byte("unknown_bucket")).Bucket([]byte("nested")).Get([]byte("opaque")), []byte{0, 1, 255}) {
			t.Error("opaque content changed")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	reportJSON, err := reportBytes(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{fixtureAuth, f.account.Credentials.Email, f.account.Credentials.Password, f.account.Credentials.TOTPSecret} {
		if bytes.Contains(reportJSON, []byte(private)) {
			t.Fatal("report exposed identity or credentials")
		}
	}
}

func TestUpgradeRejectsUnresolvedJobsAndConflictsWithoutDestination(t *testing.T) {
	for _, mode := range []string{"queued", "running", "exchanging", "awaiting_commit", "committing", "cancelling", "commit_unknown", "saved_not_adopted", "unknown", "schema", "missing_auth", "identity", "principal", "email", "duplicate_identity_bytes", "unsafe_auth", "vault_conflict", "duplicate_json"} {
		t.Run(mode, func(t *testing.T) {
			f := makeFixture(t)
			switch mode {
			case "missing_auth":
				os.Remove(filepath.Join(f.options.AuthDir, fixtureAuth))
			case "duplicate_json":
				os.WriteFile(filepath.Join(f.options.AuthDir, fixtureAuth), []byte(`{"type":"codex","type":"codex"}`), 0600)
			default:
				mutateSource(t, f, func(tx *bolt.Tx) error {
					if mode == "schema" {
						return tx.Bucket([]byte("meta")).Put([]byte("schema"), []byte("1"))
					}
					if mode == "vault_conflict" {
						v := f.vault
						v.Identity = otherIdentity
						return tx.Bucket([]byte("vault")).Put([]byte(fixtureAuth), gobBytes(t, v))
					}
					a := f.account
					switch mode {
					case "identity":
						a.View.IdentityVersion = "not-a-uuid"
					case "principal":
						a.Principal = "conflicting-principal"
					case "email":
						a.Credentials.Email = "other@example.test"
						a.EmailHash = emailHash(bytes.Repeat([]byte{42}, 32), a.Credentials.Email)
					case "duplicate_identity_bytes":
						a.FutureFields["repeat"] = oldIdentity
					case "unsafe_auth":
						a.View.AuthID = "../outside.json"
					default:
						j := f.job
						j.View.Status = mode
						return tx.Bucket([]byte("jobs")).Put([]byte(j.View.ID), gobBytes(t, j))
					}
					return tx.Bucket([]byte("accounts")).Put([]byte(a.View.ID), gobBytes(t, a))
				})
			}
			before, _ := fileHash(f.options.Source)
			_, err := upgrade(f.options)
			if err == nil {
				t.Fatal("invalid source accepted")
			}
			if _, err = os.Lstat(f.options.Destination); !os.IsNotExist(err) {
				t.Fatal("rejected migration published destination")
			}
			after, _ := fileHash(f.options.Source)
			if before != after {
				t.Fatal("rejected conversion changed source")
			}
		})
	}
}

func TestUpgradeNeverOverwritesAndRejectsSymlinks(t *testing.T) {
	for _, mode := range []string{"destination_exists", "credential_symlink", "source_symlink", "destination_permissions"} {
		t.Run(mode, func(t *testing.T) {
			f := makeFixture(t)
			switch mode {
			case "destination_exists":
				os.WriteFile(f.options.Destination, []byte("existing bytes"), 0600)
			case "credential_symlink":
				path := filepath.Join(f.options.AuthDir, fixtureAuth)
				raw, _ := os.ReadFile(path)
				outside := filepath.Join(filepath.Dir(f.options.AuthDir), "outside.json")
				os.WriteFile(outside, raw, 0600)
				os.Remove(path)
				os.Symlink(outside, path)
			case "source_symlink":
				path := f.options.Source + ".link"
				os.Symlink(f.options.Source, path)
				f.options.Source = path
			case "destination_permissions":
				os.Chmod(filepath.Dir(f.options.Destination), 0755)
			}
			if _, err := upgrade(f.options); err == nil {
				t.Fatal("unsafe destination/source accepted")
			}
			if mode == "destination_exists" {
				raw, _ := os.ReadFile(f.options.Destination)
				if string(raw) != "existing bytes" {
					t.Fatal("existing target overwritten")
				}
			}
		})
	}
}

func TestGobPatchPreservesUnknownFieldsAndRejectsAmbiguity(t *testing.T) {
	original := struct {
		Identity string
		Unknown  map[string][]byte
	}{oldIdentity, map[string][]byte{"opaque": bytes.Repeat([]byte("payload"), 1000)}}
	raw := gobBytes(t, original)
	next := strings.Repeat("a", 64)
	patched, err := replaceGobString(raw, oldIdentity, next)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Identity string
		Unknown  map[string][]byte
	}
	if err = decodeGob(patched, &got); err != nil {
		t.Fatal(err)
	}
	original.Identity = next
	if !reflect.DeepEqual(original, got) {
		t.Fatal("unknown fields changed")
	}
	ambiguous := gobBytes(t, struct{ A, B string }{oldIdentity, oldIdentity})
	if _, err = replaceGobString(ambiguous, oldIdentity, next); err == nil {
		t.Fatal("ambiguous replacement accepted")
	}
}

func TestCredentialJSONRejectsNestedDuplicateKeys(t *testing.T) {
	for _, raw := range []string{
		`{"policy":{"weight":1,"weight":2}}`,
		`{"policy":[{"key":1,"\u006bey":2}]}`,
		`{"type":"codex"} {"type":"codex"}`,
	} {
		if err := uniqueJSON([]byte(raw)); err == nil {
			t.Fatal("ambiguous credential JSON accepted")
		}
	}
	if err := uniqueJSON([]byte(`{"policy":[{"weight":9007199254740993123456789}],"type":"codex"}`)); err != nil {
		t.Fatal("valid precise policy JSON rejected")
	}
}
