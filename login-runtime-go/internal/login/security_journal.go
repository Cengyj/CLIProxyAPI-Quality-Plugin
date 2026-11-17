package login

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"

	"github.com/MoeForge/moego/ojson"
)

// SecurityJournal is durable operation evidence, never a resumable web session.
// Pending mutation intents are deliberately retained after process interruption.
type SecurityJournal struct {
	Version                int    `json:"version"`
	JobID                  string `json:"job_id"`
	Attempt                int    `json:"attempt"`
	Operation              string `json:"operation"`
	Email                  string `json:"email"`
	Phase                  string `json:"phase"`
	Status                 string `json:"status"`
	AccountID              string `json:"account_id"`
	OldFactorID            string `json:"old_factor_id"`
	NewFactorID            string `json:"new_factor_id"`
	NewTOTPSecret          string `json:"new_totp_secret"`
	EnrollmentSessionID    string `json:"enrollment_session_id"`
	DisableStatus          string `json:"disable_status"`
	EnrollStatus           string `json:"enroll_status"`
	ActivationStatus       string `json:"activation_status"`
	ActivationConfirmation string `json:"activation_confirmation"`
	LogoutStatus           string `json:"logout_status"`
}

type securityJournal struct {
	SecurityJournal
	path        string
	hash        [32]byte
	initialized bool
}

func newSecurityJournal(in Input) (*securityJournal, error) {
	j := &securityJournal{SecurityJournal: SecurityJournal{Version: 1, JobID: in.JobID, Attempt: in.Attempt, Operation: in.Operation, Email: in.Credentials.Email, Status: "running", DisableStatus: "not_requested", EnrollStatus: "not_requested", ActivationStatus: "not_requested", LogoutStatus: "not_requested"}, path: in.SecurityJournalPath}
	if err := j.save("security_check"); err != nil {
		return nil, err
	}
	return j, nil
}

func (j *securityJournal) save(phase string) error {
	failed := func() error { return failure("security_storage_failed", phase, "journal_unavailable") }
	dir := filepath.Dir(j.path)
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return failed()
	}
	// Resolve the parent once on each publication to reject symlink redirection.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || resolved != dir {
		return failed()
	}
	if j.initialized {
		previous, err := readPrivateJournal(j.path)
		if err != nil || sha256.Sum256(previous) != j.hash {
			return failed()
		}
	} else if _, err := os.Lstat(j.path); !errors.Is(err, os.ErrNotExist) {
		return failed()
	}
	j.Phase = phase
	raw, err := encodePayload(j.SecurityJournal)
	if err != nil {
		return failed()
	}
	defer clear(raw)
	temp, err := os.CreateTemp(dir, ".security-journal-")
	if err != nil {
		return failed()
	}
	name := temp.Name()
	defer func() { _ = temp.Close(); _ = os.Remove(name) }()
	if err = temp.Chmod(0600); err != nil {
		return failed()
	}
	if _, err = temp.Write(raw); err != nil {
		return failed()
	}
	if err = temp.Sync(); err != nil {
		return failed()
	}
	if err = temp.Close(); err != nil {
		return failed()
	}
	if !j.initialized {
		// Hard linking publishes without ever replacing another operation's evidence.
		if err = os.Link(name, j.path); err != nil {
			return failed()
		}
		if err = os.Remove(name); err != nil {
			return failed()
		}
	} else if err = os.Rename(name, j.path); err != nil {
		return failed()
	}
	directory, err := os.Open(dir)
	if err != nil {
		return failed()
	}
	syncErr := directory.Sync()
	closeErr := directory.Close()
	if syncErr != nil || closeErr != nil {
		return failed()
	}
	saved, err := readPrivateJournal(j.path)
	if err != nil || !bytes.Equal(saved, raw) {
		return failed()
	}
	clear(saved)
	j.hash = sha256.Sum256(raw)
	j.initialized = true
	return nil
}

func readPrivateJournal(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 32<<10 {
		return nil, errors.New("invalid private journal")
	}
	raw, err := os.ReadFile(path)
	if err != nil || !ojson.ParseBytes(raw).IsObject() {
		return nil, errors.New("invalid private journal")
	}
	return raw, nil
}

func (j *securityJournal) finishFailure() error {
	if j.Status == "succeeded" {
		return nil
	}
	j.Status = "failed"
	for _, state := range []string{j.DisableStatus, j.EnrollStatus, j.ActivationStatus, j.LogoutStatus} {
		if state == "pending" || state == "unknown" {
			j.Status = "unknown"
		}
	}
	return j.save(j.Phase)
}
func (j *securityJournal) currentSecret(previous string) string {
	if j.ActivationStatus == "succeeded" && j.NewTOTPSecret != "" {
		return j.NewTOTPSecret
	}
	return previous
}
