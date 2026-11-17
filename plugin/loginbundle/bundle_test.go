package loginbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/MoeForge/moego/ojson"
)

type archiveEntry struct {
	name string
	data []byte
	kind byte
}

func fixtureArchive(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		kind := entry.kind
		if kind == 0 {
			kind = tar.TypeReg
		}
		header := &tar.Header{Name: entry.name, Mode: 0o600, Size: int64(len(entry.data)), Typeflag: kind}
		if kind != tar.TypeReg {
			header.Size = 0
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if kind == tar.TypeReg {
			if _, err := tw.Write(entry.data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func fixtureBundle(t *testing.T) (bundleSpec, []archiveEntry) {
	t.Helper()
	entrypoint := "bin/login-runtime"
	if runtime.GOOS == "windows" {
		entrypoint += ".exe"
	}
	program := []byte("offline native helper fixture\n")
	license := []byte("fixture license\n")
	doc := ojson.NewDocString(`{}`)
	for key, value := range map[string]any{
		"schema_version": 2, "runtime_version": "2.0.1", "engine": "go_sobek_protocol",
		"platform": runtime.GOOS, "arch": runtime.GOARCH, "entrypoint": entrypoint,
		"files": map[string]string{entrypoint: digest(program), "licenses/example@v1.0.0/LICENSE": digest(license)},
	} {
		if err := doc.Set(key, value); err != nil {
			t.Fatal(err)
		}
	}
	manifest := doc.Bytes()
	entries := []archiveEntry{{"manifest.json", manifest, 0}, {entrypoint, program, 0}, {"licenses/example@v1.0.0/LICENSE", license, 0}}
	archive := fixtureArchive(t, entries)
	return bundleSpec{archive: archive, sha256: digest(archive), manifestSHA256: digest(manifest), version: "2.0.1", platform: runtime.GOOS, arch: runtime.GOARCH}, entries
}

func TestBundleInstallsAndReusesImmutablePrivateDirectory(t *testing.T) {
	spec, _ := fixtureBundle(t)
	private := t.TempDir()
	manifest, err := ensureBundle(private, spec)
	if err != nil {
		t.Fatal(err)
	}
	again, err := ensureBundle(private, spec)
	if err != nil || again != manifest {
		t.Fatalf("existing bundle was not reused: %v", err)
	}
	if !strings.Contains(manifest, "2.0.1-"+runtime.GOOS+"-"+runtime.GOARCH+"-"+spec.sha256[:12]) {
		t.Fatal("bundle path does not identify its version and digest")
	}
	if runtime.GOOS != "windows" {
		for _, name := range []string{private, filepath.Dir(manifest), manifest} {
			info, err := os.Stat(name)
			if err != nil || info.Mode().Perm()&0o077 != 0 {
				t.Fatal("bundle has permissions for another user")
			}
		}
	}
	root := filepath.Dir(manifest)
	program := filepath.Join(root, "bin", "login-runtime")
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	changed := []byte("changed fixture\n")
	if err := os.WriteFile(program, changed, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err = ensureBundle(private, spec); !errors.Is(err, ErrBundleIntegrity) {
		t.Fatalf("tampered installed helper accepted: %v", err)
	}
	data, _ := os.ReadFile(program)
	if !bytes.Equal(data, changed) {
		t.Fatal("existing installation was overwritten")
	}
}

func TestBundleRejectsCorruptionAndIncompleteArchives(t *testing.T) {
	for _, name := range []string{"archive-digest", "manifest-digest", "wrong-platform", "missing-file", "extra-file", "tampered-file", "duplicate-file", "traversal", "symlink", "hardlink"} {
		t.Run(name, func(t *testing.T) {
			spec, entries := fixtureBundle(t)
			switch name {
			case "archive-digest":
				spec.sha256 = strings.Repeat("0", 64)
			case "manifest-digest":
				spec.manifestSHA256 = strings.Repeat("0", 64)
			case "wrong-platform":
				spec.platform = "unsupported"
			case "missing-file":
				entries = entries[:2]
			case "extra-file":
				entries = append(entries, archiveEntry{"unlisted", []byte("extra"), 0})
			case "tampered-file":
				entries[1].data = []byte("changed")
			case "duplicate-file":
				entries = append(entries, entries[1])
			case "traversal":
				entries = append(entries, archiveEntry{"../outside", []byte("extra"), 0})
			case "symlink":
				entries = append(entries, archiveEntry{"linked", nil, tar.TypeSymlink})
			case "hardlink":
				entries = append(entries, archiveEntry{"linked", nil, tar.TypeLink})
			}
			if name != "archive-digest" {
				spec.archive = fixtureArchive(t, entries)
				spec.sha256 = digest(spec.archive)
			}
			private := t.TempDir()
			if _, err := ensureBundle(private, spec); !errors.Is(err, ErrBundleIntegrity) {
				t.Fatalf("invalid bundle accepted: %v", err)
			}
			stages, _ := filepath.Glob(filepath.Join(private, "login-runtime", ".install-*"))
			if len(stages) != 0 {
				t.Fatal("failed extraction left a staging directory")
			}
		})
	}
}

func TestBundleConcurrentInstallationsConverge(t *testing.T) {
	spec, _ := fixtureBundle(t)
	private := t.TempDir()
	var group sync.WaitGroup
	errorsFound := make(chan error, 8)
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := ensureBundle(private, spec)
			errorsFound <- err
		}()
	}
	group.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(private, "login-runtime"))
	if err != nil || len(entries) != 1 {
		t.Fatal("concurrent extraction left duplicate or staging installations")
	}
}

func TestSourceBuildReportsMissingBundle(t *testing.T) {
	if _, err := ensureBundle(t.TempDir(), bundleSpec{}); !errors.Is(err, ErrBundleUnavailable) {
		t.Fatalf("missing bundle did not produce an actionable failure: %v", err)
	}
}
