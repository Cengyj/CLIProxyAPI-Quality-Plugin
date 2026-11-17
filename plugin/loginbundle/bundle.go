// Package loginbundle installs the embedded native login engine without a
// separate host installation step or any external JavaScript interpreter.
package loginbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/MoeForge/moego/ojson"
)

var ErrBundleUnavailable = errors.New("native login bundle is not embedded; build with scripts/build_official_plugin.py")
var ErrBundleIntegrity = errors.New("native login bundle failed integrity verification")

const maxFileBytes = 128 << 20
const maxBundleBytes = 256 << 20
const maxBundleFiles = 256
const maxManifestBytes = 256 << 10

type bundleSpec struct {
	archive                 []byte
	sha256, manifestSHA256  string
	version, platform, arch string
}

var installMu sync.Mutex

// Ensure returns the verified manifest of this plugin's embedded engine. A
// digest-addressed installation is immutable: a corrupt existing directory is
// rejected, never repaired underneath an already running worker.
func Ensure(dataDir string) (string, error) {
	installMu.Lock()
	defer installMu.Unlock()
	return ensureBundle(dataDir, embeddedSpec())
}

func ensureBundle(dataDir string, spec bundleSpec) (string, error) {
	if len(spec.archive) == 0 || spec.sha256 == "" {
		return "", ErrBundleUnavailable
	}
	if len(spec.archive) > maxBundleBytes || digest(spec.archive) != spec.sha256 || !validDigest(spec.manifestSHA256) || spec.platform != runtime.GOOS || spec.arch != runtime.GOARCH || !safeSegment(spec.version) {
		return "", ErrBundleIntegrity
	}
	if strings.TrimSpace(dataDir) == "" {
		return "", errors.New("native login data directory is required")
	}
	base, err := filepath.Abs(dataDir)
	if err != nil {
		return "", err
	}
	if err = makePrivateDir(base); err != nil {
		return "", err
	}
	root := filepath.Join(base, "login-runtime")
	if err = makePrivateDir(root); err != nil {
		return "", err
	}
	name := spec.version + "-" + spec.platform + "-" + spec.arch + "-" + spec.sha256[:12]
	destination := filepath.Join(root, name)
	if _, err = os.Lstat(destination); err == nil {
		return verifyInstallation(destination, spec)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	stage, err := os.MkdirTemp(root, ".install-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	if err = unpack(stage, spec.archive); err != nil {
		return "", err
	}
	if _, err = verifyInstallation(stage, spec); err != nil {
		return "", err
	}
	if err = os.Rename(stage, destination); err != nil {
		// Another process can finish the identical immutable installation first.
		if _, statErr := os.Lstat(destination); statErr == nil {
			return verifyInstallation(destination, spec)
		}
		return "", fmt.Errorf("install native login bundle: %w", err)
	}
	return filepath.Join(destination, "manifest.json"), nil
}

func makePrivateDir(directory string) error {
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrBundleIntegrity
	}
	return os.Chmod(directory, 0o700)
}

func safeSegment(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, ch := range value {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '.' || ch == '-' || ch == '_' || ch == '@' || ch == '+') {
			return false
		}
	}
	return true
}

func safePath(name string) bool {
	if name == "" || path.IsAbs(name) || path.Clean(name) != name || strings.ContainsAny(name, "\\:\x00") {
		return false
	}
	for _, segment := range strings.Split(name, "/") {
		if !safeSegment(segment) {
			return false
		}
	}
	return true
}

func validDigest(value string) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == sha256.Size && value == strings.ToLower(value)
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func unpack(root string, archive []byte) error {
	zipped, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return ErrBundleIntegrity
	}
	defer zipped.Close()
	reader := tar.NewReader(zipped)
	seen := map[string]bool{}
	var total int64
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || !safePath(header.Name) || seen[header.Name] || len(seen) >= maxBundleFiles || header.Typeflag != tar.TypeReg || header.Size < 0 || header.Size > maxFileBytes {
			return ErrBundleIntegrity
		}
		seen[header.Name] = true
		total += header.Size
		if total > maxBundleBytes {
			return ErrBundleIntegrity
		}
		target := filepath.Join(root, filepath.FromSlash(header.Name))
		if err = makePrivateDir(filepath.Dir(target)); err != nil {
			return err
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return ErrBundleIntegrity
		}
		_, copyErr := io.CopyN(file, reader, header.Size)
		syncErr := file.Sync()
		closeErr := file.Close()
		if copyErr != nil || syncErr != nil || closeErr != nil {
			return ErrBundleIntegrity
		}
		if header.Name == "bin/login-runtime" || header.Name == "bin/login-runtime.exe" {
			if err = os.Chmod(target, 0o700); err != nil {
				return err
			}
		}
	}
	// Consume the gzip trailer so a corrupt checksum cannot hide after tar EOF.
	if trailing, err := io.Copy(io.Discard, io.LimitReader(zipped, maxBundleBytes+1)); err != nil || trailing > maxBundleBytes {
		return ErrBundleIntegrity
	}
	return nil
}

func verifyInstallation(root string, spec bundleSpec) (string, error) {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrBundleIntegrity
	}
	manifestPath := filepath.Join(root, "manifest.json")
	info, err = os.Lstat(manifestPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxManifestBytes {
		return "", ErrBundleIntegrity
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil || digest(data) != spec.manifestSHA256 || !ojson.ValidBytes(data) {
		return "", ErrBundleIntegrity
	}
	manifest := ojson.ParseBytes(data)
	entrypoint := "bin/login-runtime"
	if spec.platform == "windows" {
		entrypoint += ".exe"
	}
	if manifest.Get("schema_version").Int() != 2 || manifest.Get("runtime_version").String() != spec.version || manifest.Get("engine").String() != "go_sobek_protocol" || manifest.Get("platform").String() != spec.platform || manifest.Get("arch").String() != spec.arch || manifest.Get("entrypoint").String() != entrypoint || !manifest.Get("files").IsObject() {
		return "", ErrBundleIntegrity
	}
	files := map[string]string{"manifest.json": spec.manifestSHA256}
	valid := true
	manifest.Get("files").ForEach(func(key, value ojson.Result) bool {
		name, hash := key.String(), value.String()
		if !safePath(name) || files[name] != "" || value.Type != ojson.String || !validDigest(hash) || len(files) >= maxBundleFiles {
			valid = false
			return false
		}
		files[name] = hash
		return true
	})
	if !valid || files[entrypoint] == "" {
		return "", ErrBundleIntegrity
	}
	seen := 0
	err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return ErrBundleIntegrity
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
			return ErrBundleIntegrity
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, current)
		name := filepath.ToSlash(relative)
		if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileBytes || files[name] == "" {
			return ErrBundleIntegrity
		}
		file, err := os.Open(current)
		if err != nil {
			return err
		}
		hash := sha256.New()
		_, copyErr := io.Copy(hash, file)
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil || hex.EncodeToString(hash.Sum(nil)) != files[name] {
			return ErrBundleIntegrity
		}
		if name == entrypoint && runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0 {
			return ErrBundleIntegrity
		}
		seen++
		return nil
	})
	if err != nil || seen != len(files) {
		return "", ErrBundleIntegrity
	}
	return manifestPath, nil
}
