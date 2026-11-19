package account

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	json "github.com/MoeForge/moego/json"
	"github.com/MoeForge/moego/ojson"
	"github.com/cliproxyapi-plugins/model-quality/login/conversion"
)

// ImportConverted only creates new files. Receipts consume an identity before
// the host write, so a lost response or restart cannot silently repeat a save.
func (m *Manager) ImportConverted(ctx context.Context, item conversion.Item, guard func() error, prepare func(context.Context, string, string) error) (LoginCommitResult, error) {
	m.commitMu.Lock()
	defer m.commitMu.Unlock()
	result := LoginCommitResult{CommitID: "conversion:" + conversion.Identity(item), AuthID: item.Filename, Principal: conversion.Identity(item), State: "aborted"}
	if item.Status != "ready" || item.Credential == nil || item.Filename == "" {
		return result, errors.New("invalid_request")
	}
	principal, err := LoginPrincipalFromAuth(&Auth{Provider: "codex", Metadata: item.Credential})
	if err != nil {
		return result, err
	}
	if principal.AccountID != item.AccountID || principal.UserID != item.UserID {
		return result, errors.New("identity_conflict")
	}
	doc := ojson.NewDocString(`{}`)
	for key, value := range item.Credential {
		if err := doc.Set(key, value); err != nil {
			return result, errors.New("invalid_credentials")
		}
	}
	canonical, err := canonicalJSON(doc.Bytes())
	if err != nil {
		return result, err
	}
	digest := sha256.Sum256([]byte(canonical))
	result.Principal = hex.EncodeToString(digest[:])
	if old, err := m.LoginCommitStatus(result.CommitID); err == nil {
		if old.State == "committed" && old.Principal == result.Principal {
			return old, nil
		}
		if old.State == "committed" {
			return result, errors.New("duplicate_identity")
		}
		if old.State != "aborted" {
			return old, errors.New("commit_unknown")
		}
	} else if err.Error() != "not_found" {
		return result, errors.New("storage_unavailable")
	}
	check := func() error {
		if err := ctx.Err(); err != nil {
			return errors.New("cancelled")
		}
		if !m.Valid() {
			return errors.New("plugin_unavailable")
		}
		if guard != nil {
			return guard()
		}
		return nil
	}
	checkExisting := func() error {
		entries, err := m.entries()
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Name == item.Filename || entry.ID == item.Filename {
				return errors.New("duplicate_identity")
			}
			if entry.Provider != "codex" && entry.Type != "codex" {
				continue
			}
			if item.Email != "" && strings.EqualFold(entry.Email, item.Email) {
				return errors.New("duplicate_identity")
			}
			if entry.RuntimeOnly {
				continue
			}
			current, err := m.read(entry)
			if err != nil {
				return err
			}
			if item.Email != "" && strings.EqualFold(authMetadataString(current, "email"), item.Email) {
				return errors.New("duplicate_identity")
			}
			principal, principalErr := LoginPrincipalFromAuth(current)
			if principalErr == nil && (item.Email != "" && principal.Email == item.Email || principal.AccountID == item.AccountID && (item.UserID == "" || principal.UserID == item.UserID)) {
				return errors.New("duplicate_identity")
			}
			if authMetadataString(current, "account_id") == item.AccountID && (item.UserID == "" || authMetadataString(current, "user_id") == item.UserID || principalErr != nil) {
				return errors.New("duplicate_identity")
			}
		}
		return nil
	}
	if err := check(); err != nil {
		return result, err
	}
	if err := checkExisting(); err != nil {
		return result, err
	}
	if prepare != nil {
		if err := prepare(ctx, item.Filename, ""); err != nil {
			return result, err
		}
	}
	if err := check(); err != nil {
		return result, err
	}
	if err := checkExisting(); err != nil {
		return result, err
	}
	result.State = "commit_unknown"
	if err := m.receipt(result); err != nil {
		return result, errors.New("storage_unavailable")
	}
	if err := check(); err != nil {
		result.State = "aborted"
		if saveErr := m.receipt(result); saveErr != nil {
			result.State = "commit_unknown"
			return result, errors.New("commit_unknown")
		}
		return result, err
	}
	var saved struct {
		Name string `json:"name"`
	}
	if err := m.invoke("host.auth.save", map[string]any{"name": item.Filename, "json": json.RawMessage(doc.Bytes())}, &saved); err != nil {
		return result, errors.New("commit_unknown")
	}
	if saved.Name != item.Filename {
		return result, errors.New("commit_unknown")
	}
	entries, err := m.entries()
	if err != nil {
		return result, errors.New("commit_unknown")
	}
	for _, entry := range entries {
		if entry.Name != item.Filename {
			continue
		}
		actual, err := m.read(entry)
		if err != nil || actual.Provider != "codex" {
			return result, errors.New("commit_unknown")
		}
		for key, value := range item.Credential {
			if actual.Metadata[key] != value {
				return result, errors.New("commit_unknown")
			}
		}
		var live struct {
			Auth Entry `json:"auth"`
		}
		if err := m.invoke("host.auth.get_runtime", map[string]string{"auth_index": actual.Index}, &live); err != nil {
			return result, errors.New("saved_not_adopted")
		}
		disabled, _ := item.Credential["disabled"].(bool)
		provider := live.Auth.Provider
		if provider == "" {
			provider = live.Auth.Type
		}
		if live.Auth.ID != actual.ID || live.Auth.Index != actual.Index || provider != "codex" || live.Auth.Status == "" || (live.Auth.Disabled || live.Auth.Status == "disabled") != disabled {
			return result, errors.New("saved_not_adopted")
		}
		result.AuthID, result.State, result.CredentialEnabled = actual.ID, "committed", !disabled
		if err := m.receipt(result); err != nil {
			result.State = "commit_unknown"
			return result, errors.New("commit_unknown")
		}
		return result, nil
	}
	return result, errors.New("saved_not_adopted")
}
