package login

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MoeForge/moego/ojson"
	codex "github.com/cliproxyapi-plugins/model-quality/login/oauth"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
	bolt "go.etcd.io/bbolt"
)

func TestManualReloginReplacesOnlyAfterSuccessfulExchange(t *testing.T) {
	host := newFakeHost()
	opts := offlineOptions(t)
	var attempt atomic.Int32
	var offline atomic.Bool
	opts.Call = func(method string, in, out any) error {
		if offline.Load() {
			return errors.New("offline")
		}
		return host.call(method, in, out)
	}
	opts.Exchange = func(context.Context, string, *codex.PKCECodes, string) (*codex.CodexAuthBundle, error) {
		n := attempt.Add(1)
		if n == 5 {
			return nil, errors.New("exchange_failed")
		}
		bundle := offlineBundle(t, time.Now(), "offline-account")
		bundle.TokenData.RefreshToken = fmt.Sprintf("fixture-%d", n)
		return bundle, nil
	}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	startOfflineAccount(t, s, a, "", "committed")
	if _, err := s.Accounts(); err != nil {
		t.Fatal(err)
	}
	offline.Store(true)
	var previous Job
	for n := 2; n <= 5; n++ {
		detail, err := s.AccountDetail(a.ID)
		if err != nil {
			t.Fatal(err)
		}
		job, err := s.startManualAccount(a.ID, detail.Account.Revision, "", false)
		if err != nil {
			t.Fatal(err)
		}
		want := "awaiting_commit"
		if n == 5 {
			want = "failed"
		}
		job = waitLogin(t, s, job.ID, want)
		token := fmt.Sprintf("fixture-%d", min(n, 4))
		if err := s.journal.db.View(func(tx *bolt.Tx) error {
			raw := tx.Bucket([]byte("account-authorizations")).Get([]byte(a.ID))
			if ojson.GetBytes(raw, "refresh_token").String() != token {
				return errors.New("wrong local grant")
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if n < 5 {
			if previous.ID != "" {
				if _, err := s.Candidate(previous.CandidateID); SafeCode(err) != "not_found" {
					t.Fatal("old candidate remained uploadable")
				}
			}
			previous = job
		}
	}
	detail, err := s.AccountDetail(a.ID)
	if err != nil || detail.Account.PendingUploadJobID != previous.ID || host.saves != 1 {
		t.Fatal("failed relogin lost previous result or wrote to CPA")
	}
	offline.Store(false)
	commitCandidate(t, s, previous)
	detail, err = s.AccountDetail(a.ID)
	if err != nil || detail.Account.LastJobID != previous.ID || host.saves != 2 || host.files[detail.Account.AuthID]["refresh_token"] != "fixture-4" {
		t.Fatal("manual upload did not adopt the last successful grant")
	}
}

func TestAccountAuthorizationWithoutUploadWaitsForConfirmation(t *testing.T) {
	for _, reset := range []bool{false, true} {
		name := "login"
		if reset {
			name = "reset"
		}
		t.Run(name, func(t *testing.T) {
			host := newFakeHost()
			opts := offlineOptions(t)
			opts.Call = host.call
			if reset {
				opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
					if err := writeSecurityFixture(in, securityFixtureJournal(in)); err != nil {
						return "", err
					}
					return offlineCallback(in.State), nil
				}}
			}
			s := offlineService(t, opts)
			a := importOfflineAccount(t, s)
			body := map[string]any{"id": a.ID, "revision": a.Revision, "upload_to_cpa": false}
			route := "/accounts/start"
			if reset {
				route = "/accounts/security"
				body["action"], body["logout_all_devices"], body["reauthorize"] = "reset_totp", true, true
			}
			raw, _ := Encode(body)
			response := s.Handle(context.Background(), pluginapi.ManagementRequest{Method: http.MethodPost, Path: RoutePrefix + route, Body: raw, Headers: http.Header{"Idempotency-Key": {"manual-upload-fixture"}}})
			if response.StatusCode != 200 {
				t.Fatalf("start returned %d: %s", response.StatusCode, response.Body)
			}
			jobs := s.Jobs()
			if len(jobs) != 1 {
				t.Fatalf("expected one job, got %d", len(jobs))
			}
			job := waitLogin(t, s, jobs[0].ID, "awaiting_commit")
			host.mu.Lock()
			savedBefore := host.saves
			host.mu.Unlock()
			if savedBefore != 0 || job.AutoCommit || !job.EnableCredential {
				t.Fatal("authorization wrote to CPA or did not allow an explicit upload")
			}
			if reset {
				detail, err := s.AccountDetail(a.ID)
				if err != nil || detail.Account.SecurityPending || detail.Credentials.TOTPSecret != securityFixtureSeed {
					t.Fatal("new 2FA seed was not saved before manual upload")
				}
			}
			candidate, err := s.Candidate(job.CandidateID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Commit(context.Background(), job.ID, candidate.ID, candidate.Revision); err != nil {
				t.Fatal(err)
			}
			job = waitLogin(t, s, job.ID, "committed")
			host.mu.Lock()
			defer host.mu.Unlock()
			if host.saves != 1 || !job.CredentialEnabled {
				t.Fatal("confirmed upload did not save and enable exactly once")
			}
		})
	}
}
