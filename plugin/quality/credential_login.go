package quality

import (
	"context"
	"errors"

	"github.com/MoeForge/moego/ojson"
	bolt "go.etcd.io/bbolt"
)

type CredentialLoginPrepareRequest struct {
	AuthID           string `json:"auth_id"`
	ExpectedIdentity string `json:"expected_identity"`
	PreparationID    string `json:"preparation_id"`
}

type CredentialLoginPrepareResponse struct {
	PreparationID string `json:"preparation_id"`
	Ready         bool   `json:"ready"`
}

// PrepareCredentialLogin durably invalidates old evidence before the host writes
// a new grant. Existing pause ownership remains untouched and rules stay stopped.
func (en *Engine) PrepareCredentialLogin(ctx context.Context, request CredentialLoginPrepareRequest) (CredentialLoginPrepareResponse, error) {
	reply := CredentialLoginPrepareResponse{PreparationID: request.PreparationID}
	if request.AuthID == "" || len(request.AuthID) > 512 || len(request.ExpectedIdentity) > 128 || request.PreparationID == "" || len(request.PreparationID) > 128 {
		return reply, errors.New("invalid login preparation")
	}
	en.mu.Lock()
	if err := en.mutableLocked(); err != nil {
		en.mu.Unlock()
		return reply, err
	}
	if !en.Capabilities().QualityPause && en.localPause(RunConfig{AuthID: request.AuthID}) != "" {
		en.mu.Unlock()
		return reply, unsupportedQuality()
	}
	var prepared bool
	err := en.store.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte("meta")).Get([]byte("login-prepare:" + request.PreparationID))
		if raw == nil {
			return nil
		}
		if ojson.GetBytes(raw, "auth_id").String() != request.AuthID || ojson.GetBytes(raw, "identity").String() != request.ExpectedIdentity {
			return errors.New("login preparation conflict")
		}
		prepared = true
		return nil
	})
	if err != nil {
		en.mu.Unlock()
		return reply, err
	}
	if !prepared {
		c := changes()
		d := ojson.NewDocString(`{}`)
		_ = d.Set("auth_id", request.AuthID)
		_ = d.Set("identity", request.ExpectedIdentity)
		c.loginPreparationID = request.PreparationID
		c.loginPreparation = d.Bytes()
		for id, old := range en.rules {
			if !usesCredential(old.RunConfig, request.AuthID) {
				continue
			}
			r := cloneRule(old)
			r.Enabled = false
			r.Revision++
			resetRule(r)
			r.Status = ruleStatus(r)
			c.rules[id] = r
		}
		for id, old := range en.runs {
			if !usesCredential(old.Config.RunConfig, request.AuthID) || (old.Status != "running" && old.Status != "queued") {
				continue
			}
			r := cloneRun(old)
			r.CancelRequested = true
			r.BlockReason = "credential_authorization_changed"
			if r.Status == "queued" {
				r.Status = "cancelled"
				now := en.now()
				r.CompletedAt = &now
			}
			c.runs[id] = r
		}
		c.audits = append(c.audits, en.audit("credential_login_prepare", "", "", request.PreparationID, "Dependent rules stopped; credential restrictions preserved"))
		if err := en.commitLocked(c); err != nil {
			en.mu.Unlock()
			return reply, err
		}
	}
	for {
		if err := ctx.Err(); err != nil {
			en.mu.Unlock()
			return reply, err
		}
		if err := en.mutableLocked(); err != nil {
			en.mu.Unlock()
			return reply, err
		}
		if en.ctx.Err() != nil {
			en.mu.Unlock()
			return reply, errors.New("plugin stopping")
		}
		if _, busy := en.busyAuth[request.AuthID]; !busy {
			en.mu.Unlock()
			reply.Ready = true
			return reply, nil
		}
		if en.queueChanged == nil {
			en.notifyWorkersLocked()
		}
		changed := en.queueChanged
		en.mu.Unlock()
		select {
		case <-ctx.Done():
			return reply, ctx.Err()
		case <-en.ctx.Done():
			return reply, errors.New("plugin stopping")
		case <-changed:
		}
		en.mu.Lock()
	}
}
