package quality

import (
	"fmt"
	"slices"
	"sort"

	"github.com/MoeForge/moego/json"
	bolt "go.etcd.io/bbolt"
)

const defaultMaxResults = 100

type historyClearResult struct {
	Deleted int `json:"deleted"`
	Kept    int `json:"kept"`
}

func (en *Engine) clearRuleHistory() (historyClearResult, error) {
	en.mu.Lock()
	defer en.mu.Unlock()
	var result historyClearResult
	if err := en.mutableLocked(); err != nil {
		return result, err
	}
	drop, liveBatches := map[string]bool{}, map[string]bool{}
	for id, run := range en.runs {
		if run.Source != "scheduled_probe" {
			continue
		}
		r := en.rules[run.Config.RuleID]
		if run.CompletedAt == nil || en.active[id] != nil || (r != nil && (r.ActionID != "" || r.BlockedAuthID != "")) {
			liveBatches[run.Config.ScopeBatchID] = true
			result.Kept++
			continue
		}
		drop[id] = true
	}
	updates := map[string]*Rule{}
	for id, r := range en.rules {
		clearBatch := r.LastScopeBatch != "" && !liveBatches[r.LastScopeBatch]
		if !drop[r.LastRun] && !clearBatch {
			continue
		}
		n := cloneRule(r)
		if drop[r.LastRun] {
			n.LastRun = ""
		}
		if clearBatch {
			n.LastScopeBatch = ""
		}
		updates[id] = n
	}
	if len(drop) == 0 && len(updates) == 0 {
		return result, nil
	}
	err := en.store.Write(func(tx *bolt.Tx) error {
		for id, r := range updates {
			if err := put(tx, "rules", id, r); err != nil {
				return err
			}
		}
		return deleteRuleRecords(tx, drop, "")
	})
	if err != nil {
		return historyClearResult{}, err
	}
	for id := range drop {
		delete(en.runs, id)
	}
	for id, r := range updates {
		*en.rules[id] = *r
	}
	result.Deleted = len(drop)
	return result, nil
}

func deleteRuleRecords(tx *bolt.Tx, drop map[string]bool, deletedRule string) error {
	for id := range drop {
		if err := tx.Bucket([]byte("runs")).Delete([]byte(id)); err != nil {
			return err
		}
	}
	b := tx.Bucket([]byte("audit"))
	var keys []string
	if err := b.ForEach(func(k, v []byte) error {
		var a Audit
		if err := json.Unmarshal(v, &a); err != nil {
			return err
		}
		if drop[a.RunID] || (deletedRule != "" && a.RuleID == deletedRule) {
			keys = append(keys, string(k))
		}
		return nil
	}); err != nil {
		return err
	}
	for _, k := range keys {
		if err := b.Delete([]byte(k)); err != nil {
			return err
		}
	}
	return nil
}

func prepareResultLimit(in, old *Rule) error {
	if in.RetentionDays != 0 && (old == nil || old.RetentionDays != in.RetentionDays) {
		return fmt.Errorf("invalid_request: use max_results instead of retention_days")
	}
	if in.RetentionMode == "" && in.MaxResults == 0 && old != nil {
		in.RetentionMode = old.RetentionMode
	}
	switch in.RetentionMode {
	case "manual":
		if in.MaxResults != 0 {
			return fmt.Errorf("invalid_request: manual retention cannot specify max_results")
		}
		in.RetentionDays = 0
		return nil
	case "", "count":
	default:
		return fmt.Errorf("invalid_request: unknown retention_mode")
	}
	if in.MaxResults == 0 {
		switch {
		case old != nil && old.MaxResults > 0:
			in.MaxResults = old.MaxResults
		case old != nil && old.RetentionDays > 0 && in.RetentionMode == "":
			in.RetentionDays = old.RetentionDays
			return nil
		default:
			in.MaxResults = max(defaultMaxResults, len(in.Models)*in.Samples)
		}
	}
	if in.MaxResults < 1 || in.MaxResults > 200 || in.MaxResults < len(in.Models)*in.Samples {
		return fmt.Errorf("invalid_request: max_results must be 1..200 and fit one complete round")
	}
	in.RetentionDays = 0
	return nil
}

// Each account in a dynamic rule has its own result window. Keep sample ordinals
// stable across a partially retained round so a stale review cannot hit another sample.
func (en *Engine) pruneResultCountsLocked(ruleID, authID string) error {
	type key struct{ rule, auth string }
	groups := make(map[key][]*Run)
	for _, r := range en.runs {
		if r.Source != "scheduled_probe" || r.CompletedAt == nil || r.Status == "running" || r.Status == "queued" || (ruleID != "" && (r.Config.RuleID != ruleID || r.Config.AuthID != authID)) {
			continue
		}
		rule := en.rules[r.Config.RuleID]
		if rule == nil || rule.MaxResults < 1 || rule.ActionID != "" || rule.BlockedAuthID != "" {
			continue
		}
		k := key{r.Config.RuleID, r.Config.AuthID}
		groups[k] = append(groups[k], r)
	}
	var drops []string
	var trims []*Run
	for k, list := range groups {
		rule := en.rules[k.rule]
		remaining := rule.MaxResults
		sort.Slice(list, func(i, j int) bool {
			if list[i].CompletedAt.Equal(*list[j].CompletedAt) {
				return list[i].ID > list[j].ID
			}
			return list[i].CompletedAt.After(*list[j].CompletedAt)
		})
		for _, r := range list {
			weight := max(1, len(r.Samples)) // An unsuccessful run without a sample is one diagnostic result.
			switch {
			case remaining >= weight:
				remaining -= weight
			case remaining == 0:
				if rule.LastRun != r.ID {
					drops = append(drops, r.ID)
				}
			default:
				n := cloneRun(r)
				n.PrunedSamples += weight - remaining
				n.Samples = slices.Clone(n.Samples[weight-remaining:])
				n.Controls = slices.DeleteFunc(n.Controls, func(c Sample) bool {
					return !slices.ContainsFunc(n.Samples, func(s Sample) bool { return s.Model == c.Model && s.Index == c.Index })
				})
				trims = append(trims, n)
				remaining = 0
			}
		}
	}
	if len(trims) == 0 && len(drops) == 0 {
		return nil
	}
	if err := en.store.Write(func(tx *bolt.Tx) error {
		for _, r := range trims {
			if err := put(tx, "runs", r.ID, r); err != nil {
				return err
			}
		}
		for _, id := range drops {
			if err := tx.Bucket([]byte("runs")).Delete([]byte(id)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, r := range trims {
		en.runs[r.ID] = r
	}
	for _, id := range drops {
		delete(en.runs, id)
	}
	return nil
}
