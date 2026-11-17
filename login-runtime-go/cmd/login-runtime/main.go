package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/MoeForge/moego/ojson"
	"github.com/cli-proxy-api/model-quality-login-runtime/internal/authmaterial"
	"github.com/cli-proxy-api/model-quality-login-runtime/internal/jshost"
	"github.com/cli-proxy-api/model-quality-login-runtime/internal/login"
	"github.com/grafana/sobek"
)

const version = "2.1.0"
const engine = "go_sobek_protocol"
const inputLimit = 16 << 10

type loginRunner func(context.Context, login.Input, func(string)) (string, error)

func main() {
	// CPA owns the outer process group and must be able to reap every JS worker.
	if err := os.Setenv("MQ_LOGIN_OWNED_PROCESS_GROUP", "1"); err != nil {
		os.Exit(1)
	}
	if handled, status := jshost.WorkerMain(os.Args[1:], os.Stdin, os.Stdout); handled {
		os.Exit(status)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	ctx, closeOwner, err := ownerContext(ctx, os.Getenv("MQ_LOGIN_OWNER_FD"))
	if err != nil {
		cancel()
		_ = emit(os.Stdout, map[string]any{"version": 1, "error_code": "runtime_unavailable"})
		os.Exit(1)
	}
	status := run(ctx, os.Args[1:], os.Stdin, os.Stdout, login.Run)
	closeOwner()
	cancel()
	os.Exit(status)
}

// ownerContext keeps process ownership separate from the one-shot JSON input.
// The launcher holds FD 3's write end until the job finishes; process death or
// plugin shutdown closes that end and cancels all HTTP and Sobek work.
func ownerContext(parent context.Context, descriptor string) (context.Context, func(), error) {
	ctx, cancel := context.WithCancel(parent)
	if descriptor == "" {
		return ctx, cancel, nil
	}
	if descriptor != "3" {
		cancel()
		return nil, nil, errors.New("invalid owner descriptor")
	}
	pipe := os.NewFile(3, "login-owner")
	if pipe == nil {
		cancel()
		return nil, nil, errors.New("owner descriptor unavailable")
	}
	info, err := pipe.Stat()
	if err != nil || info.Mode()&os.ModeNamedPipe == 0 {
		_ = pipe.Close()
		cancel()
		return nil, nil, errors.New("owner descriptor is not a pipe")
	}
	go func() {
		_, _ = io.Copy(io.Discard, pipe)
		cancel()
	}()
	return ctx, func() { cancel(); _ = pipe.Close() }, nil
}

func emit(out io.Writer, fields map[string]any) error {
	doc := ojson.NewDocString(`{}`)
	for name, value := range fields {
		if err := doc.Set(name, value); err != nil {
			return err
		}
	}
	_, err := out.Write(append(doc.Bytes(), '\n'))
	return err
}

func run(ctx context.Context, args []string, in io.Reader, out io.Writer, runner loginRunner) (status int) {
	identity := map[string]any{"version": 1}
	defer func() {
		if recover() != nil {
			identity["error_code"] = "runtime_unavailable"
			_ = emit(out, identity)
			status = 1
		}
	}()
	if len(args) == 1 && args[0] == "--self-test" {
		vm := sobek.New()
		value, err := vm.RunString(`JSON.stringify({engine: "sobek", value: 6 * 7})`)
		if err != nil || value.String() != `{"engine":"sobek","value":42}` {
			_ = emit(out, map[string]any{"version": 1, "error_code": "runtime_unavailable"})
			return 1
		}
		if emit(out, map[string]any{"version": 1, "runtime_version": version, "engine": engine, "platform": runtime.GOOS, "arch": runtime.GOARCH, "methods": []string{"password_totp"}, "security_operations": []string{"reset_totp", "logout_all"}, "browser_required": false, "node_required": false}) != nil {
			return 1
		}
		return 0
	}
	if len(args) != 0 {
		identity["error_code"] = "invalid_input"
		_ = emit(out, identity)
		return 1
	}
	raw, err := io.ReadAll(io.LimitReader(in, inputLimit+1))
	defer clear(raw)
	if err != nil || len(raw) > inputLimit || authmaterial.ValidateUnambiguousJSON(raw) != nil {
		identity["error_code"] = "invalid_input"
		_ = emit(out, identity)
		return 1
	}
	input, err := decodeInput(raw)
	if err != nil {
		identity["error_code"] = "invalid_input"
		_ = emit(out, identity)
		return 1
	}
	identity["job_id"], identity["attempt"] = input.JobID, input.Attempt
	if err = login.ValidateInput(input); err != nil {
		return finishError(out, identity, err)
	}
	seconds := input.TimeoutSeconds
	if seconds == 0 {
		seconds = 180
		if input.Operation != "" {
			seconds = 360
		}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()
	var progressMu sync.Mutex
	last, count := "", 0
	var outputErr error
	callback, err := runner(ctx, input, func(phase string) {
		progressMu.Lock()
		defer progressMu.Unlock()
		if !allowed(phase, "runtime_start", "session", "authorization", "email", "password", "totp", "oauth", "callback", "browser_verification", "consent", "workspace", "security_check", "totp_disable", "totp_enroll", "totp_activate", "totp_saved", "logout_all", "reauthorization", "security_complete") || phase == last || outputErr != nil {
			return
		}
		if count >= 64 {
			outputErr = errors.New("progress_limit")
			cancel()
			return
		}
		count++
		last = phase
		outputErr = emit(out, map[string]any{"version": 1, "type": "progress", "job_id": input.JobID, "attempt": input.Attempt, "phase": phase})
		if outputErr != nil {
			cancel()
		}
	})
	progressMu.Lock()
	defer progressMu.Unlock()
	if outputErr != nil {
		return 1
	}
	if err != nil {
		return finishError(out, identity, err)
	}
	if input.Operation != "" {
		identity["security_complete"] = true
	}
	if callback != "" {
		identity["callback_url"] = callback
	}
	if emit(out, identity) != nil {
		return 1
	}
	return 0
}

func finishError(out io.Writer, identity map[string]any, err error) int {
	code := "transport_error"
	var failure *login.Error
	if errors.As(err, &failure) {
		if allowed(failure.Code, "invalid_input", "invalid_totp_secret", "unsupported_totp_parameters", "runtime_unavailable", "transport_error", "timeout", "cancelled", "interaction_required", "password_rejected", "totp_rejected", "authorization_denied", "rate_limited", "service_error", "callback_invalid", "callback_state_mismatch", "workspace_mismatch", "proxy_unsupported", "security_storage_failed", "security_identity_mismatch", "security_state_unconfirmed", "security_outcome_unknown", "security_rejected") {
			code = failure.Code
		}
		if allowed(failure.Stage, "runtime_start", "session", "authorization", "email", "password", "totp", "oauth", "callback", "browser_verification", "consent", "workspace", "security_check", "totp_disable", "totp_enroll", "totp_activate", "totp_saved", "logout_all", "reauthorization", "security_complete") {
			identity["error_stage"] = failure.Stage
		}
		if allowed(failure.Reason, "human_verification", "browser_verification_timeout", "browser_verification_required", "email_verification", "phone_verification", "unsupported_request", "unsupported_navigation", "totp_method_unconfirmed", "workspace_selection", "account_selection", "unsupported_page", "upstream_rejected", "proxy_connection_failed", "session_unconfirmed", "authentication_not_advanced", "totp_factor_missing", "sentinel_unavailable", "journal_unavailable") {
			identity["error_reason"] = failure.Reason
		}
	} else if errors.Is(err, context.Canceled) {
		code = "cancelled"
	} else if errors.Is(err, context.DeadlineExceeded) {
		code = "timeout"
	}
	identity["error_code"] = code
	_ = emit(out, identity)
	return 1
}

func allowed(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

func decodeInput(raw []byte) (login.Input, error) {
	var in login.Input
	errInvalid := errors.New("invalid_input")
	object := ojson.ParseBytes(raw)
	if !object.IsObject() {
		return in, errInvalid
	}
	valid := true
	object.ForEach(func(k, v ojson.Result) bool {
		name := k.String()
		switch name {
		case "version", "attempt", "timeout_seconds":
			if v.Type != ojson.Number || v.Raw != v.String() || v.Float() != float64(v.Int()) {
				valid = false
				return false
			}
			switch name {
			case "version":
				in.Version = int(v.Int())
			case "attempt":
				in.Attempt = int(v.Int())
			case "timeout_seconds":
				in.TimeoutSeconds = int(v.Int())
			}
		case "logout_after_reset", "reauthorize":
			if v.Type != ojson.True && v.Type != ojson.False {
				valid = false
				return false
			}
			if name == "logout_after_reset" {
				in.LogoutAfterReset = v.Bool()
			} else {
				in.Reauthorize = v.Bool()
			}
		case "credentials":
			if !v.IsObject() {
				valid = false
				return false
			}
			v.ForEach(func(k, v ojson.Result) bool {
				if v.Type != ojson.String {
					valid = false
					return false
				}
				switch k.String() {
				case "email":
					in.Credentials.Email = v.String()
				case "password":
					in.Credentials.Password = v.String()
				case "totp_secret":
					in.Credentials.TOTPSecret = v.String()
				default:
					valid = false
				}
				return valid
			})
		default:
			if v.Type != ojson.String {
				valid = false
				return false
			}
			switch name {
			case "job_id":
				in.JobID = v.String()
			case "authorization_url":
				in.AuthorizationURL = v.String()
			case "redirect_uri":
				in.RedirectURI = v.String()
			case "state":
				in.State = v.String()
			case "proxy_url":
				in.ProxyURL = v.String()
			case "workspace_id":
				in.WorkspaceID = v.String()
			case "operation":
				in.Operation = v.String()
			case "security_journal_path":
				in.SecurityJournalPath = v.String()
			default:
				valid = false
			}
		}
		return valid
	})
	if !valid {
		return in, errInvalid
	}
	return in, nil
}
