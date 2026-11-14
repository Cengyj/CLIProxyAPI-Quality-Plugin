package quality

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func (en *Engine) History() ([]Audit, error) {
	out := []Audit{}
	err := en.store.All("audit", func(b []byte) error {
		var a Audit
		if e := decode(b, &a); e != nil {
			return e
		}
		out = append(out, a)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unavailable: read audit history: %w", err)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Time.Equal(out[j].Time) {
			return out[i].ID < out[j].ID
		}
		return out[i].Time.After(out[j].Time)
	})
	return out, nil
}

// QueryRuns sorts lightweight references and clones only the requested page.
func (en *Engine) QueryRuns(query url.Values) (map[string]any, error) {
	source := query.Get("source")
	if source != "" && source != "manual_test" && source != "scheduled_probe" {
		return nil, fmt.Errorf("invalid_request: source must be manual_test or scheduled_probe")
	}
	offset, limit := pageBounds(query)
	en.mu.Lock()
	defer en.mu.Unlock()
	refs := make([]*Run, 0, len(en.runs))
	for _, run := range en.runs {
		if source == "" || run.Source == source {
			refs = append(refs, run)
		}
	}
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].CreatedAt.Equal(refs[j].CreatedAt) {
			return refs[i].ID < refs[j].ID
		}
		return refs[i].CreatedAt.After(refs[j].CreatedAt)
	})
	total := len(refs)
	if offset > total {
		offset = total
	}
	end := offset + min(limit, total-offset)
	entries := make([]Run, 0, end-offset)
	for _, run := range refs[offset:end] {
		entries = append(entries, *cloneRun(run))
	}
	return map[string]any{"items": entries, "total": total, "offset": offset, "limit": limit}, nil
}

func trafficSource(source string) string {
	switch source {
	case "production", "manual_test", "scheduled_probe", "judge", "internal_other":
		return source
	default:
		return "unknown"
	}
}

func page[T any](items []T, req pluginapi.ManagementRequest) map[string]any {
	offset, limit := pageBounds(req.Query)
	total := len(items)
	if offset > total {
		offset = total
	}
	end := min(offset+limit, total)
	if items == nil {
		items = []T{}
	}
	return map[string]any{"items": items[offset:end], "total": total, "offset": offset, "limit": limit}
}

func pageBounds(query url.Values) (offset, limit int) {
	offset, _ = strconv.Atoi(query.Get("offset"))
	limit, _ = strconv.Atoi(query.Get("limit"))
	if offset < 0 {
		offset = 0
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	return offset, limit
}

func (en *Engine) Run(id string) (*Run, error) {
	en.mu.Lock()
	defer en.mu.Unlock()
	r := en.runs[id]
	if r == nil {
		return nil, fmt.Errorf("not_found: run")
	}
	return cloneRun(r), nil
}
func (en *Engine) Runs() []Run {
	en.mu.Lock()
	defer en.mu.Unlock()
	out := []Run{}
	for _, r := range en.runs {
		out = append(out, *cloneRun(r))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out
}
