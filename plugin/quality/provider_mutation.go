package quality

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"
)

// ETags cover the whole CPA configuration. Re-read and compare the selected
// unit immediately before the PUT. A concurrent write after that read yields
// 412. An unrelated earlier write does not invalidate an owned route forever.
func (h *NativeHost) setProviderDisabled(ctx context.Context, expected CredentialSnapshot, disabled bool) (CredentialSnapshot, error) {
	r, err := h.pinnedConfiguredRoute(ctx, expected.AuthID, expected.AuthIndex)
	if err != nil {
		return CredentialSnapshot{}, fmt.Errorf("%w: %v", errProviderPrecondition, err)
	}
	before := r.snapshot()
	if expected.Disabled == disabled || !sameSnapshot(expected, before) || expected.ProviderMechanism != before.ProviderMechanism || !slices.Equal(expected.ProviderMembers, r.members) {
		return CredentialSnapshot{}, errProviderPrecondition
	}
	if r.unit == "group" {
		r.group["disabled"] = disabled
	} else {
		current := configStrings(r.effective("excluded-models"))
		next := withoutWildcard(current)
		if disabled {
			next = append(next, "*")
		}
		if !disabled && (expected.ProviderExclusionSource == "inherited" || expected.ProviderExclusionSource == "null") && slices.Equal(next, configStrings(r.group["excluded-models"])) {
			if expected.ProviderExclusionSource == "null" {
				r.key["excluded-models"] = nil
			} else {
				delete(r.key, "excluded-models")
			}
		} else {
			r.key["excluded-models"] = next
		}
	}
	resp, err := h.Management.conditionalResponse(ctx, http.MethodPut, providerConfigPath+"/"+r.family, r.familyList, r.etag)
	if err != nil {
		return CredentialSnapshot{}, err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusPreconditionFailed {
		return CredentialSnapshot{}, errProviderPrecondition
	}
	if resp.StatusCode != http.StatusOK || !strongConfigTag.MatchString(resp.Header.Get("ETag")) {
		return CredentialSnapshot{}, fmt.Errorf("management_uncertain: CPA did not confirm the conditional configuration update (HTTP %d)", resp.StatusCode)
	}
	deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	confirmation := "group membership or identity did not settle"
	for {
		routes, readErr := h.providerDirectory(deadline)
		seen := make(map[string]bool, len(r.members))
		var after CredentialSnapshot
		for _, entry := range routes {
			if !slices.Contains(r.members, entry.id) {
				continue
			}
			if entry.conflict || entry.groupID != r.groupID || !slices.Equal(entry.members, r.members) || entry.disabled != disabled {
				confirmation = "group membership, configuration identity or disabled state changed"
				break
			}
			if runtimeErr := h.confirmProviderRuntime(deadline, entry); runtimeErr != nil {
				confirmation = "runtime route state did not settle"
				break
			}
			if entry.id == r.id {
				after = entry.snapshot()
			}
			seen[entry.id] = true
		}
		if readErr != nil {
			confirmation = "configuration directory could not be read"
		}
		if readErr == nil && len(seen) == len(r.members) {
			if sameCredential(expected, after) && after.Disabled == disabled {
				if disabled {
					after.ProviderExclusionSource = before.ProviderExclusionSource
				}
				return after, nil
			}
			confirmation = "resulting route identity or configuration fingerprint changed"
		}
		timer := time.NewTimer(80 * time.Millisecond)
		select {
		case <-deadline.Done():
			timer.Stop()
			return CredentialSnapshot{}, fmt.Errorf("management_uncertain: %s (%v)", confirmation, deadline.Err())
		case <-timer.C:
		}
	}
}
