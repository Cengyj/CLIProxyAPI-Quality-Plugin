//go:build loginbundle_embedded

package loginbundle

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/MoeForge/moego/ojson"
)

func TestEmbeddedEngineRunsWithoutBrowserOrInterpreter(t *testing.T) {
	manifest, err := Ensure(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	document := ojson.ParseBytes(data)
	executable := filepath.Join(filepath.Dir(manifest), filepath.FromSlash(document.Get("entrypoint").String()))
	command := exec.CommandContext(t.Context(), executable, "--self-test")
	command.Env = []string{"PATH=", "HOME=" + t.TempDir(), "TMPDIR=" + t.TempDir()}
	if runtime.GOOS == "windows" {
		command.Env = append(command.Env, "SystemRoot="+os.Getenv("SystemRoot"), "WINDIR="+os.Getenv("WINDIR"), "TEMP="+t.TempDir())
	}
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	probe := ojson.ParseBytes(output)
	if probe.Get("runtime_version").String() != generatedSpec.version || probe.Get("engine").String() != "go_sobek_protocol" || !probe.Get("browser_required").Exists() || probe.Get("browser_required").Bool() || !probe.Get("node_required").Exists() || probe.Get("node_required").Bool() {
		t.Fatal("embedded bundle did not execute the declared native engine")
	}
}
