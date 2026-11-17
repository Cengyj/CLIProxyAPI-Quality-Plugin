package main

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/MoeForge/moego/ojson"
	bolt "go.etcd.io/bbolt"
)

// Both read-only database handles stay open through publication, retaining their
// shared file locks so ordinary bbolt writers cannot add a restriction afterward.
func checkQuality(statePath, pluginPath string) (func(), error) {
	state, err := openQualityReadOnly(statePath)
	if err != nil {
		return nil, err
	}
	plugin, err := openQualityReadOnly(pluginPath)
	if err != nil {
		_ = state.Close()
		return nil, err
	}
	closeBoth := func() { _ = plugin.Close(); _ = state.Close() }
	err = state.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("quality-v1"))
		if bucket == nil {
			return errors.New("quality_state_invalid")
		}
		var value struct {
			Actions      map[string]struct{}
			LoginCommits map[string]struct{ State string }
		}
		if err := decodeGob(bucket.Get([]byte("state")), &value); err != nil {
			return errors.New("quality_state_invalid")
		}
		if len(value.Actions) != 0 {
			return errors.New("quality_pause_present")
		}
		for _, commit := range value.LoginCommits {
			// These are the only terminal receipts in the legacy host journal.
			if commit.State != "committed" && commit.State != "aborted" {
				return errors.New("quality_commit_unresolved")
			}
		}
		return nil
	})
	if err == nil {
		err = plugin.View(func(tx *bolt.Tx) error {
			for _, name := range []string{"rules", "runs"} {
				bucket := tx.Bucket([]byte(name))
				if bucket == nil {
					return errors.New("quality_plugin_invalid")
				}
				if err := bucket.ForEach(func(_, raw []byte) error {
					if err := uniqueJSON(raw); err != nil {
						return errors.New("quality_plugin_invalid")
					}
					value := ojson.ParseBytes(raw)
					if !value.IsObject() {
						return errors.New("quality_plugin_invalid")
					}
					if name == "rules" {
						for _, key := range []string{"action_id", "blocked_auth_id"} {
							field := value.Get(key)
							if field.Exists() && field.Type != ojson.String {
								return errors.New("quality_plugin_invalid")
							}
							if field.String() != "" {
								return errors.New("quality_rule_restriction_present")
							}
						}
					} else {
						status := value.Get("status")
						if status.Type != ojson.String {
							return errors.New("quality_plugin_invalid")
						}
						switch status.String() {
						case "completed", "failed", "cancelled", "interrupted":
						default:
							return errors.New("quality_run_unresolved")
						}
					}
					return nil
				}); err != nil {
					return err
				}
			}
			return nil
		})
	}
	if err != nil {
		closeBoth()
		return nil, err
	}
	return closeBoth, nil
}

func openQualityReadOnly(path string) (*bolt.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, errors.New("quality_path_invalid")
	}
	if err = noSymlinks(abs); err != nil {
		return nil, errors.New("quality_path_invalid")
	}
	info, err := os.Stat(abs)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 1<<30 {
		return nil, errors.New("quality_file_invalid")
	}
	db, err := bolt.Open(abs, 0600, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if err != nil {
		return nil, errors.New("quality_open_failed")
	}
	return db, nil
}
