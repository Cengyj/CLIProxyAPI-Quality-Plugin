//go:build darwin || linux

package login

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MoeForge/moego/ojson"
)

func nativeFixture(t *testing.T, script string) *Runtime {
	t.Helper()
	root := t.TempDir()
	_ = os.Chmod(root, 0700)
	_ = os.Mkdir(filepath.Join(root, "bin"), 0700)
	binary := filepath.Join(root, "bin/login-runtime")
	if e := os.WriteFile(binary, []byte("#!/bin/sh\n"+script), 0700); e != nil {
		t.Fatal(e)
	}
	hash, e := digestFile(binary, true)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := Encode(map[string]any{"schema_version": 2, "runtime_version": "2.0.1", "engine": "go_sobek_protocol", "platform": runtime.GOOS, "arch": runtime.GOARCH, "entrypoint": "bin/login-runtime", "files": map[string]string{"bin/login-runtime": hash}})
	path := filepath.Join(root, "manifest.json")
	if e = os.WriteFile(path, raw, 0600); e != nil {
		t.Fatal(e)
	}
	return NewRuntime(path)
}
func runtimeInput() RuntimeRequest {
	return RuntimeRequest{Version: 1, JobID: "fixture", Attempt: 1, State: "fixture-state", RedirectURI: "http://localhost:1455/auth/callback", Credentials: Credentials{Email: "fixture@example.test", Password: "private-password", TOTPSecret: "JBSWY3DPEHPK3PXP"}}
}

const nativeSuccess = `cat >/dev/null
test "$MQ_LOGIN_OWNER_FD" = 3 || exit 81
test -p /dev/fd/3 || exit 82
test -z "${HTTPS_PROXY+x}${NODE_OPTIONS+x}${MQ_LOGIN_BROWSER_PATH+x}" || exit 83
printf '%s\n' '{"version":1,"type":"progress","job_id":"fixture","attempt":1,"phase":"runtime_start"}' '{"version":1,"job_id":"fixture","attempt":1,"callback_url":"http://localhost:1455/auth/callback?code=fixture-code&state=fixture-state"}'
`

func TestPortableNativeWorkerUsesOwnerPipeAndBoundedIPC(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://private.fixture")
	t.Setenv("NODE_OPTIONS", "private-node-option")
	r := nativeFixture(t, nativeSuccess)
	var phases []string
	callback, e := r.RunWithProgress(context.Background(), runtimeInput(), func(p string) error { phases = append(phases, p); return nil })
	if e != nil || !strings.Contains(callback, "fixture-code") || len(phases) != 1 || phases[0] != "runtime_start" {
		t.Fatalf("native runtime failed: %v %v", e, phases)
	}
}
func TestPortableNativeWorkerRejectsLegacyOrChangedBundle(t *testing.T) {
	for _, mode := range []string{"legacy", "changed", "node", "symlink"} {
		t.Run(mode, func(t *testing.T) {
			r := nativeFixture(t, nativeSuccess)
			raw, _ := os.ReadFile(r.manifestPath)
			d := ojson.NewDoc(raw)
			switch mode {
			case "legacy":
				_ = d.Set("schema_version", 1)
			case "node":
				_ = d.Set("node", "bin/node")
			case "changed":
				_ = os.WriteFile(filepath.Join(filepath.Dir(r.manifestPath), "bin/login-runtime"), []byte("changed"), 0700)
			case "symlink":
				_ = os.Symlink("bin/login-runtime", filepath.Join(filepath.Dir(r.manifestPath), "other"))
			}
			_ = os.WriteFile(r.manifestPath, d.Bytes(), 0600)
			if e := r.Capabilities(); e == nil {
				t.Fatal("unsafe runtime bundle accepted")
			}
		})
	}
}

func TestNativeRuntimeVersionCapabilitiesIncludeWorkspaceSelection(t *testing.T) {
	for _, tc := range []struct {
		version  string
		login    bool
		security bool
	}{
		{version: "2.0.1", login: true, security: false},
		{version: "2.1.0", login: true, security: false},
		{version: "2.1.1", login: true, security: false},
		{version: "2.1.2", login: true, security: false},
		{version: "2.1.3", login: true, security: false},
		{version: "2.2.0", login: true, security: false},
		{version: "2.2.1", login: true, security: true},
	} {
		t.Run(tc.version, func(t *testing.T) {
			r := nativeFixture(t, "exit 99\n")
			raw, err := os.ReadFile(r.manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			doc := ojson.NewDoc(raw)
			if err := doc.Set("runtime_version", tc.version); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(r.manifestPath, doc.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			if err := r.Capabilities(); (err == nil) != tc.login {
				t.Fatalf("runtime version %s login capability: %v", tc.version, err)
			}
			opts := offlineOptions(t)
			opts.Runtime = r
			s := offlineService(t, opts)
			capabilities, err := Encode(s.Capabilities())
			if err != nil {
				t.Fatal(err)
			}
			if ojson.GetBytes(capabilities, "methods.password_totp.available").Bool() != tc.login ||
				ojson.GetBytes(capabilities, "security_actions.reset_totp").Bool() != tc.security ||
				ojson.GetBytes(capabilities, "security_actions.logout_all").Bool() != tc.security {
				t.Fatalf("runtime version %s published incorrect login/security capabilities", tc.version)
			}
			if tc.login && ojson.GetBytes(capabilities, "runtime.runtime_version").String() != tc.version {
				t.Fatal("verified runtime version was not published")
			}
		})
	}
}
func TestPortableNativeWorkerCancellationClosesOwner(t *testing.T) {
	r := nativeFixture(t, "cat >/dev/null\ncat <&3 >/dev/null\nexit 0\n")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, e := r.Run(ctx, runtimeInput())
	if e == nil || e.Error() != "runtime_timeout" || time.Since(started) > 3*time.Second {
		t.Fatalf("worker cancellation did not finish: %v", e)
	}
}

func TestNativeCommitPipeAcknowledgesOnlyAfterPersistence(t *testing.T) {
	script := `cat >/dev/null
test "$MQ_LOGIN_COMMIT_FD" = 4 || exit 81
printf '%s\n' '{"version":1,"type":"factor_verified","job_id":"fixture","attempt":1,"factor_id":"verified-factor"}'
dd bs=1 count=1 <&4 >/dev/null 2>&1 || exit 82
printf '%s\n' '{"version":1,"type":"security_commit","job_id":"fixture","attempt":1}'
dd bs=1 count=1 <&4 >/dev/null 2>&1 || exit 83
printf '%s\n' '{"version":1,"type":"progress","job_id":"fixture","attempt":1,"phase":"logout_all"}' '{"version":1,"job_id":"fixture","attempt":1,"security_complete":true}'
`
	r := nativeFixture(t, script)
	request := runtimeInput()
	request.Operation = "reset_totp"
	entered, release := make(chan struct{}), make(chan struct{})
	verified := false
	request.factorVerified = func(id string) error { verified = id == "verified-factor"; return nil }
	request.commitSecurity = func() error { close(entered); <-release; return nil }
	logout := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		_, err := r.RunWithProgress(t.Context(), request, func(phase string) error {
			if phase == "logout_all" {
				logout <- struct{}{}
			}
			return nil
		})
		done <- err
	}()
	<-entered
	select {
	case <-logout:
		t.Error("child continued before credential acknowledgement")
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !verified || len(logout) != 1 {
		t.Fatal("private acknowledgement lost factor or continuation")
	}
}

func TestNativeCommitFailureBlocksContinuation(t *testing.T) {
	r := nativeFixture(t, `cat >/dev/null
printf '%s\n' '{"version":1,"type":"security_commit","job_id":"fixture","attempt":1}'
dd bs=1 count=1 <&4 >/dev/null 2>&1
printf '%s\n' '{"version":1,"type":"progress","job_id":"fixture","attempt":1,"phase":"logout_all"}'
`)
	request := runtimeInput()
	request.Operation = "reset_totp"
	request.commitSecurity = func() error { return runtimeError("storage_unavailable") }
	progressed := false
	_, err := r.RunWithProgress(t.Context(), request, func(string) error { progressed = true; return nil })
	if err == nil || err.Error() != "storage_unavailable" || progressed {
		t.Fatal("failed commit was acknowledged")
	}
}
