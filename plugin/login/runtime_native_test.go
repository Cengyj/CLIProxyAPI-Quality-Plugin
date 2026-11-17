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
