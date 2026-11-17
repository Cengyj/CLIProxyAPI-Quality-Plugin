package login

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/MoeForge/moego/ojson"
)

const runtimeInputLimit = 16 << 10
const runtimeOutputLimit = 12 << 10
const runtimeStreamLimit = 64 << 10
const runtimeProgressLimit = 64
const runtimeManifestLimit = 512 << 10

// Runtime executes a local, administrator-installed authorization worker.
// No secrets are placed in command arguments, environment, or diagnostic errors.
type Runtime struct{ manifestPath string }

func NewRuntime(manifestPath string) *Runtime { return &Runtime{manifestPath: manifestPath} }

type runtimeManifest struct {
	root, executable, browser string
	engine, version, schema   string
	arguments                 []string
	progress                  bool
}

func runtimeError(code string) error { return errors.New(code) }

type runtimeDiagnosticError struct {
	code, stage, reason string
}

func (e *runtimeDiagnosticError) Error() string { return e.code }

func runtimeDiagnostic(value string, allowed []string) bool {
	for _, candidate := range allowed {
		if candidate == value {
			return true
		}
	}
	return false
}

func runtimeStage(value string) bool {
	return runtimeDiagnostic(value, []string{"security_login", "security_check", "totp_disable", "totp_enroll", "totp_activate", "totp_saved", "logout_all", "reauthorization", "security_complete", "runtime_start", "browser_start", "session", "authorization", "email", "password", "totp", "oauth", "callback", "browser_verification", "consent", "workspace"})
}

func runtimeReason(value string) bool {
	return runtimeDiagnostic(value, []string{"journal_unavailable", "human_verification", "browser_verification_timeout", "browser_verification_required", "email_verification", "phone_verification", "unsupported_request", "unsupported_navigation", "submit_unavailable", "totp_method_unconfirmed", "workspace_selection", "account_selection", "unexpected_dialog", "unexpected_download", "unsupported_page", "upstream_rejected", "proxy_connection_failed", "session_unconfirmed", "authentication_not_advanced", "totp_factor_missing", "sentinel_unavailable"})
}

func (r *Runtime) Capabilities() error {
	_, err := r.verify()
	return err
}

// Metadata reports the verified installed engine without exposing local paths.
func (r *Runtime) Metadata() (map[string]string, error) {
	manifest, err := r.verify()
	if err != nil {
		return nil, err
	}
	return map[string]string{"engine": manifest.engine, "runtime_version": manifest.version, "schema_version": manifest.schema}, nil
}

func digestFile(filePath string, executable bool) (string, error) {
	return digestRegularFile(filePath, executable, 0022)
}

func digestRegularFile(filePath string, executable bool, forbiddenWrites os.FileMode) (string, error) {
	info, err := os.Lstat(filePath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&forbiddenWrites != 0 || (executable && info.Mode().Perm()&0111 == 0) || info.Size() > 512<<20 {
		return "", runtimeError("runtime_invalid")
	}
	f, err := os.Open(filePath)
	if err != nil {
		return "", runtimeError("runtime_invalid")
	}
	h := sha256.New()
	_, errCopy := io.Copy(h, io.LimitReader(f, 512<<20+1))
	errClose := f.Close()
	if errCopy != nil || errClose != nil {
		return "", runtimeError("runtime_invalid")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func validDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && strings.ToLower(value) == value
}

func (r *Runtime) verify() (runtimeManifest, error) {
	var out runtimeManifest
	if !runtimeSupported() || r == nil || !filepath.IsAbs(r.manifestPath) {
		return out, runtimeError("runtime_unavailable")
	}
	out.root = filepath.Dir(r.manifestPath)
	info, err := os.Lstat(out.root)
	if err != nil {
		return out, runtimeError("runtime_unavailable")
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 || filepath.Base(r.manifestPath) != "manifest.json" {
		return out, runtimeError("runtime_invalid")
	}
	info, err = os.Lstat(r.manifestPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > runtimeManifestLimit {
		return out, runtimeError("runtime_invalid")
	}
	raw, err := os.ReadFile(r.manifestPath)
	if err != nil || len(raw) > runtimeManifestLimit {
		return out, runtimeError("runtime_invalid")
	}
	manifest, err := Object(raw, "schema_version", "runtime_version", "engine", "platform", "arch", "node", "entrypoint", "files", "browser")
	version := manifest.Get("runtime_version").String()
	if err != nil || manifest.Get("platform").String() != runtime.GOOS || manifest.Get("arch").String() != runtime.GOARCH {
		return out, runtimeError("runtime_invalid")
	}
	var executable string
	var required []string
	if manifest.Get("schema_version").Raw != "2" || (version != "2.0.0" && version != "2.0.1" && version != "2.1.0") || manifest.Get("engine").String() != "go_sobek_protocol" || manifest.Get("entrypoint").String() != "bin/login-runtime" || manifest.Get("node").Exists() || manifest.Get("browser").Exists() {
		return out, runtimeError("runtime_invalid")
	}
	executable = "bin/login-runtime"
	out.progress = true
	required = []string{executable}
	out.engine = manifest.Get("engine").String()
	out.version = version
	out.schema = manifest.Get("schema_version").Raw
	files := manifest.Get("files")
	if !files.IsObject() {
		return out, runtimeError("runtime_invalid")
	}
	declared := make(map[string]string)
	valid := true
	files.ForEach(func(key, value ojson.Result) bool {
		name := key.String()
		if name == "" || name == "." || name == "manifest.json" || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\:\x00") || value.Type != ojson.String || !validDigest(value.String()) || declared[name] != "" || len(declared) >= 2048 {
			valid = false
			return false
		}
		declared[name] = value.String()
		return true
	})
	for _, name := range required {
		if declared[name] == "" {
			valid = false
		}
	}
	if !valid {
		return out, runtimeError("runtime_invalid")
	}
	seen := make(map[string]bool)
	err = filepath.WalkDir(out.root, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.Type()&os.ModeSymlink != 0 {
			return runtimeError("runtime_invalid")
		}
		if entry.IsDir() {
			stat, errInfo := entry.Info()
			if errInfo != nil || stat.Mode().Perm()&0022 != 0 {
				return runtimeError("runtime_invalid")
			}
			return nil
		}
		rel, errRel := filepath.Rel(out.root, filePath)
		if errRel != nil {
			return runtimeError("runtime_invalid")
		}
		name := filepath.ToSlash(rel)
		if name == "manifest.json" {
			return nil
		}
		expected := declared[name]
		actual, errDigest := digestFile(filePath, name == executable)
		if expected == "" || errDigest != nil || expected != actual {
			return runtimeError("runtime_invalid")
		}
		seen[name] = true
		return nil
	})
	if err != nil || len(seen) != len(declared) {
		return out, runtimeError("runtime_invalid")
	}
	out.executable = filepath.Join(out.root, filepath.FromSlash(executable))
	return out, nil
}

type limitedRuntimeOutput struct {
	mu       sync.Mutex
	data     bytes.Buffer
	limit    int
	overflow chan struct{}
}

func (b *limitedRuntimeOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	available := b.limit - b.data.Len()
	if available > len(p) {
		available = len(p)
	}
	if available > 0 {
		_, _ = b.data.Write(p[:available])
	}
	if available < len(p) {
		select {
		case b.overflow <- struct{}{}:
		default:
		}
	}
	return len(p), nil
}

// runtimeProgressOutput accepts only fixed, identity-bound progress events and
// one final response. Its bounded buffer never retains previous progress lines.
type runtimeProgressOutput struct {
	mu       sync.Mutex
	pending  []byte
	final    []byte
	total    int
	count    int
	ended    bool
	err      error
	failed   chan error
	request  RuntimeRequest
	progress func(string) error
}

func (b *runtimeProgressOutput) reject(err error) {
	if b.err != nil {
		return
	}
	b.err = err
	select {
	case b.failed <- err:
	default:
	}
}

func runtimeIdentity(result ojson.Result, request RuntimeRequest) bool {
	return result.Get("version").Raw == "1" && result.Get("job_id").Type == ojson.String && result.Get("job_id").String() == request.JobID && result.Get("attempt").Type == ojson.Number && result.Get("attempt").Int() == int64(request.Attempt) && result.Get("attempt").Float() == float64(request.Attempt)
}

func (b *runtimeProgressOutput) line(raw []byte) {
	if b.err != nil {
		return
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return
	}
	if len(raw) > runtimeOutputLimit || b.ended {
		b.reject(runtimeError("runtime_protocol_error"))
		return
	}
	result, err := Object(raw, "version", "type", "job_id", "attempt", "phase", "callback_url", "error_code", "error_stage", "error_reason", "security_complete")
	if err != nil || !runtimeIdentity(result, b.request) {
		b.reject(runtimeError("runtime_protocol_error"))
		return
	}
	if result.Get("type").Exists() {
		progress, err := Object(raw, "version", "type", "job_id", "attempt", "phase")
		if err != nil || progress.Get("type").Type != ojson.String || progress.Get("type").String() != "progress" || progress.Get("phase").Type != ojson.String || !runtimeStage(progress.Get("phase").String()) || b.count >= runtimeProgressLimit {
			b.reject(runtimeError("runtime_protocol_error"))
			return
		}
		b.count++
		if b.progress != nil {
			if err := b.progress(progress.Get("phase").String()); err != nil {
				b.reject(err)
			}
		}
		return
	}
	if result.Get("phase").Exists() {
		b.reject(runtimeError("runtime_protocol_error"))
		return
	}
	b.final = append(b.final, raw...)
	b.ended = true
}

func (b *runtimeProgressOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return len(p), nil
	}
	b.total += len(p)
	if b.total > runtimeStreamLimit {
		b.reject(runtimeError("runtime_protocol_error"))
		return len(p), nil
	}
	b.pending = append(b.pending, p...)
	for {
		index := bytes.IndexByte(b.pending, '\n')
		if index < 0 {
			break
		}
		b.line(b.pending[:index])
		clear(b.pending[:index+1])
		b.pending = b.pending[index+1:]
	}
	if len(b.pending) > runtimeOutputLimit {
		b.reject(runtimeError("runtime_protocol_error"))
	}
	return len(p), nil
}

func (b *runtimeProgressOutput) finish() ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.line(b.pending)
	clear(b.pending)
	b.pending = nil
	if b.err != nil {
		return nil, b.err
	}
	if !b.ended {
		return nil, runtimeError("runtime_protocol_error")
	}
	return b.final, nil
}

func (b *runtimeProgressOutput) clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	clear(b.pending)
	clear(b.final)
}

func (r *Runtime) Run(ctx context.Context, request RuntimeRequest) (string, error) {
	return r.RunWithProgress(ctx, request, nil)
}

func (r *Runtime) RunWithProgress(ctx context.Context, request RuntimeRequest, progress func(string) error) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", runtimeError("cancelled")
	}
	manifest, err := r.verify()
	if err != nil {
		return "", err
	}
	raw, err := Encode(request)
	if err != nil || len(raw) > runtimeInputLimit {
		return "", runtimeError("runtime_protocol_error")
	}
	defer clear(raw)
	timeout := 190 * time.Second
	if request.Operation != "" {
		timeout = 370 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tmp, err := os.MkdirTemp("", "credential-login-")
	if err != nil {
		return "", runtimeError("runtime_unavailable")
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	cmd := exec.Command(manifest.executable, manifest.arguments...)
	cmd.Dir = manifest.root
	cmd.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + tmp, "TMPDIR=" + tmp, "TMP=" + tmp, "TEMP=" + tmp, "LANG=en_US.UTF-8"}
	ownerRead, ownerWrite, err := os.Pipe()
	if err != nil {
		return "", runtimeError("runtime_unavailable")
	}
	defer ownerRead.Close()
	defer ownerWrite.Close()
	cmd.ExtraFiles = []*os.File{ownerRead}
	cmd.Env = append(cmd.Env, "MQ_LOGIN_OWNER_FD=3")
	stopOwner := context.AfterFunc(ctx, func() { _ = ownerWrite.Close() })
	defer stopOwner()
	cmd.Stdin = bytes.NewReader(raw)
	overflow := make(chan struct{}, 1)
	stdout := &limitedRuntimeOutput{limit: runtimeOutputLimit, overflow: overflow}
	stderr := &limitedRuntimeOutput{limit: runtimeOutputLimit, overflow: overflow}
	defer func() { clear(stdout.data.Bytes()); clear(stderr.data.Bytes()) }()
	cmd.Stdout, cmd.Stderr = stdout, stderr
	stream := &runtimeProgressOutput{failed: make(chan error, 1), request: RuntimeRequest{Version: request.Version, JobID: request.JobID, Attempt: request.Attempt}, progress: progress}
	if manifest.progress {
		cmd.Stdout = stream
	}
	defer stream.clear()
	cmd.WaitDelay = 2 * time.Second
	if err := configureRuntimeProcess(cmd); err != nil {
		return "", runtimeError("runtime_unavailable")
	}
	if err := cmd.Start(); err != nil {
		return "", runtimeError("runtime_unavailable")
	}
	_ = ownerRead.Close()
	defer killRuntimeProcess(cmd, true)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var runErr error
	var progressErr error
	code := ""
	select {
	case runErr = <-done:
	case <-overflow:
		code = "runtime_protocol_error"
	case progressErr = <-stream.failed:
		code = "runtime_protocol_error"
	case <-ctx.Done():
		code = "cancelled"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			code = "runtime_timeout"
		}
	}
	if code != "" {
		_ = ownerWrite.Close()
		killRuntimeProcess(cmd, false)
		timer := time.NewTimer(2 * time.Second)
		select {
		case <-done:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			killRuntimeProcess(cmd, true)
			<-done
		}
		if progressErr != nil {
			return "", progressErr
		}
		return "", runtimeError(code)
	}
	select {
	case <-overflow:
		return "", runtimeError("runtime_protocol_error")
	default:
	}
	if manifest.progress {
		result, err := stream.finish()
		if err != nil {
			return "", err
		}
		return runtimeResponse(result, request, runErr)
	}
	return runtimeResponse(stdout.data.Bytes(), request, runErr)
}

func runtimeResponse(raw []byte, request RuntimeRequest, runErr error) (string, error) {
	result, err := Object(raw, "version", "job_id", "attempt", "callback_url", "error_code", "error_stage", "error_reason", "security_complete")
	if err != nil || result.Get("version").Raw != "1" {
		return "", runtimeError("runtime_protocol_error")
	}
	if result.Get("error_code").Exists() {
		if result.Get("error_code").Type != ojson.String || result.Get("callback_url").Exists() {
			return "", runtimeError("runtime_protocol_error")
		}
		codes := map[string]string{"invalid_input": "runtime_protocol_error", "proxy_unsupported": "proxy_auth_unsupported", "invalid_totp_secret": "invalid_credentials", "unsupported_totp_parameters": "invalid_credentials", "runtime_unavailable": "runtime_unavailable", "browser_unavailable": "runtime_unavailable", "browser_failed": "runtime_failed", "transport_error": "runtime_failed", "timeout": "runtime_timeout", "cancelled": "cancelled", "interaction_required": "interaction_required", "password_rejected": "credentials_rejected", "totp_rejected": "totp_rejected", "authorization_denied": "interaction_required", "rate_limited": "rate_limited", "service_error": "runtime_failed", "callback_invalid": "invalid_callback", "callback_state_mismatch": "invalid_callback", "workspace_mismatch": "identity_conflict", "security_failed": "security_failed", "security_uncertain": "security_uncertain", "security_journal_invalid": "security_journal_invalid", "security_storage_failed": "storage_unavailable", "security_identity_mismatch": "identity_conflict", "security_state_unconfirmed": "security_failed", "security_outcome_unknown": "security_uncertain", "security_rejected": "security_failed"}
		code := codes[result.Get("error_code").String()]
		if code == "" {
			code = "runtime_protocol_error"
		}
		stage, reason := result.Get("error_stage"), result.Get("error_reason")
		if stage.Exists() && (stage.Type != ojson.String || !runtimeStage(stage.String())) {
			return "", runtimeError("runtime_protocol_error")
		}
		if reason.Exists() && (reason.Type != ojson.String || !runtimeReason(reason.String())) {
			return "", runtimeError("runtime_protocol_error")
		}
		if stage.Exists() || reason.Exists() {
			return "", &runtimeDiagnosticError{code: code, stage: stage.String(), reason: reason.String()}
		}
		return "", runtimeError(code)
	}
	if request.Operation != "" {
		if !runtimeIdentity(result, request) || result.Get("security_complete").Type != ojson.True {
			return "", runtimeError("runtime_protocol_error")
		}
		if !request.Reauthorize {
			if runErr != nil || result.Get("callback_url").Exists() || result.Get("error_stage").Exists() || result.Get("error_reason").Exists() {
				return "", runtimeError("runtime_protocol_error")
			}
			return "", nil
		}
	} else if result.Get("security_complete").Exists() {
		return "", runtimeError("runtime_protocol_error")
	}
	if runErr != nil || result.Get("error_stage").Exists() || result.Get("error_reason").Exists() || result.Get("job_id").Type != ojson.String || result.Get("job_id").String() != request.JobID || result.Get("attempt").Type != ojson.Number || result.Get("attempt").Int() != int64(request.Attempt) || result.Get("attempt").Float() != float64(request.Attempt) || result.Get("callback_url").Type != ojson.String {
		return "", runtimeError("runtime_protocol_error")
	}
	callback := result.Get("callback_url").String()
	value, err := url.Parse(callback)
	target, targetErr := url.Parse(request.RedirectURI)
	if err != nil || targetErr != nil || len(callback) > 8192 || value.Scheme != target.Scheme || value.Host != target.Host || value.Path != target.Path || value.User != nil || value.Fragment != "" {
		return "", runtimeError("invalid_callback")
	}
	query, err := url.ParseQuery(value.RawQuery)
	if err != nil || len(query["state"]) != 1 || query.Get("state") != request.State || len(query["code"]) != 1 || query.Get("code") == "" || len(query.Get("code")) > 4096 || len(query["error"]) != 0 {
		return "", runtimeError("invalid_callback")
	}
	return callback, nil
}
