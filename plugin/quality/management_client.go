package quality

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MoeForge/moego/json"
	"github.com/MoeForge/moego/ojson"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

// ManagementClient holds an authenticated panel session in memory only. It never
// discovers keys from host files, persists them, follows redirects, or uses a proxy.
type ManagementClient struct {
	mu     sync.RWMutex
	base   string
	key    string
	client *http.Client
}

func NewManagementClient(base string) (*ManagementClient, error) {
	if base == "" {
		base = "http://127.0.0.1:8317"
	}
	u, err := url.Parse(base)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid local CPA management URL")
	}
	ip := net.ParseIP(u.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return nil, fmt.Errorf("CPA management URL must use a loopback IP")
	}
	transport := &http.Transport{DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, TLSHandshakeTimeout: 5 * time.Second}
	return &ManagementClient{base: strings.TrimRight(base, "/"), client: &http.Client{
		Transport: transport, Timeout: 70 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (m *ManagementClient) authorize(headers http.Header) {
	key := strings.TrimSpace(headers.Get("X-Management-Key"))
	if value := headers.Get("Authorization"); len(value) > 7 && strings.EqualFold(value[:7], "Bearer ") {
		key = strings.TrimSpace(value[7:])
	}
	if key == "" || len(key) > 8192 || strings.ContainsAny(key, "\r\n") {
		return
	}
	m.mu.Lock()
	m.key = key
	m.mu.Unlock()
}

func (m *ManagementClient) ready() bool { m.mu.RLock(); defer m.mu.RUnlock(); return m.key != "" }

// Auth model discovery returns IDs only. Join the display page to the
// corresponding CPA channel catalog without adding models to any credential.
func (m *ManagementClient) addModelReasoning(ctx context.Context, targets []Target) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	type thinkingOptions struct {
		Levels         []string `json:"levels,omitempty"`
		ZeroAllowed    bool     `json:"zero_allowed,omitempty"`
		DynamicAllowed bool     `json:"dynamic_allowed,omitempty"`
	}
	byChannel := make(map[string]map[string]*thinkingOptions)
	for i := range targets {
		target := &targets[i]
		channel := strings.ToLower(strings.TrimSpace(target.Provider))
		if len(target.Models) == 0 || channel == "" || channel == "." || channel == ".." || strings.ContainsAny(channel, "/\\") {
			continue
		}
		catalog, loaded := byChannel[channel]
		if !loaded {
			var reply struct {
				Channel string `json:"channel"`
				Models  []struct {
					ID       string           `json:"id"`
					Thinking *thinkingOptions `json:"thinking"`
				} `json:"models"`
			}
			byChannel[channel] = nil
			if err := m.request(ctx, http.MethodGet, "/v8/management/routing/model-definitions/"+url.PathEscape(channel), nil, &reply, 4<<20); err != nil || reply.Channel != channel {
				continue
			}
			catalog = make(map[string]*thinkingOptions)
			for _, model := range reply.Models {
				catalog[model.ID] = model.Thinking
			}
			byChannel[channel] = catalog
		}
		reasoning := make(map[string]*thinkingOptions)
		for _, id := range target.Models {
			if support := catalog[id]; support != nil {
				reasoning[id] = support
			}
		}
		target.Reasoning = reasoning
	}
}

func (m *ManagementClient) response(ctx context.Context, method, path string, body any) (*http.Response, error) {
	if m == nil {
		return nil, fmt.Errorf("unavailable: CPA management connection is not configured")
	}
	m.mu.RLock()
	key := m.key
	m.mu.RUnlock()
	if key == "" {
		return nil, fmt.Errorf("management_session_required: open the authenticated plugin panel after restarting CPA")
	}
	var raw []byte
	var err error
	if body != nil {
		raw, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, m.base+path, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid management request")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("management_uncertain: CPA request did not return a confirmed result")
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		m.mu.Lock()
		if m.key == key {
			m.key = ""
		}
		m.mu.Unlock()
	}
	return resp, nil
}

func (m *ManagementClient) request(ctx context.Context, method, path string, body any, out any, limit int64) error {
	resp, err := m.response(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("management_http_%d: CPA management request failed", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return fmt.Errorf("management_uncertain: invalid or oversized CPA response")
	}
	if out != nil && json.Unmarshal(raw, out) != nil {
		return fmt.Errorf("management_uncertain: invalid CPA response")
	}
	return nil
}

// Snapshots contain hashes and public routing identity, never token material.
// UpdatedAt is an additional interference check, not an atomic ownership token.
type CredentialSnapshot struct {
	AuthID         string    `json:"auth_id"`
	AuthIndex      string    `json:"auth_index"`
	Name           string    `json:"name"`
	Provider       string    `json:"provider"`
	Fingerprint    string    `json:"fingerprint"`
	UpdatedAt      time.Time `json:"updated_at"`
	FileModifiedAt time.Time `json:"file_modified_at"`
	Disabled       bool      `json:"disabled"`
}

type managedHost interface {
	CredentialSnapshot(context.Context, string, string) (CredentialSnapshot, error)
	SetCredentialDisabled(context.Context, CredentialSnapshot, bool) (CredentialSnapshot, error)
	DeleteCredential(context.Context, CredentialSnapshot) error
}

func sameCredential(a, b CredentialSnapshot) bool {
	return a.AuthID == b.AuthID && a.AuthIndex == b.AuthIndex && a.Name == b.Name && a.Provider == b.Provider && a.Fingerprint != "" && a.Fingerprint == b.Fingerprint
}
func sameSnapshot(a, b CredentialSnapshot) bool {
	return sameCredential(a, b) && a.Disabled == b.Disabled && !a.UpdatedAt.IsZero() && a.UpdatedAt.Equal(b.UpdatedAt) && a.FileModifiedAt.Equal(b.FileModifiedAt)
}

func (h *NativeHost) credentialProfile(ctx context.Context, id, index string) (CredentialSnapshot, pluginapi.HostAuthFileEntry, map[string]any, error) {
	var runtime pluginapi.HostAuthGetRuntimeResponse
	var raw pluginapi.HostAuthGetResponse
	var empty CredentialSnapshot
	if id == "" || index == "" {
		return empty, runtime.Auth, nil, fmt.Errorf("blocked: credential identity is missing")
	}
	if err := h.call(ctx, "host.auth.get_runtime", pluginapi.HostAuthGetRequest{AuthIndex: index}, &runtime); err != nil {
		return empty, runtime.Auth, nil, fmt.Errorf("blocked: credential no longer exists")
	}
	a := runtime.Auth
	provider := a.Provider
	if provider == "" {
		provider = a.Type
	}
	if a.ID != id || a.AuthIndex != index || a.RuntimeOnly || a.Source != "file" || a.Name == "" || filepath.Base(a.Name) != a.Name || strings.ContainsAny(a.Name, "/\\") || !strings.HasSuffix(strings.ToLower(a.Name), ".json") || a.UpdatedAt.IsZero() {
		return empty, a, nil, fmt.Errorf("unsupported: automatic handling requires one identifiable file credential")
	}
	if provider != "codex" && provider != "openai" && provider != "openai-compatibility" {
		return empty, a, nil, fmt.Errorf("unsupported: management probes currently support OpenAI and Codex file credentials")
	}
	if err := h.call(ctx, "host.auth.get", pluginapi.HostAuthGetRequest{AuthIndex: index}, &raw); err != nil {
		return empty, a, nil, fmt.Errorf("blocked: credential snapshot is unavailable")
	}
	var data map[string]any
	if raw.AuthIndex != index || raw.Name != a.Name || json.Unmarshal(raw.JSON, &data) != nil || data == nil {
		return empty, a, nil, fmt.Errorf("blocked: credential snapshot identity changed")
	}
	// Expanded sources, SIWC, and ambiguous routing need their native executor.
	for _, field := range []string{"use_siwc", "virtual", "projects", "project_ids"} {
		if value := data[field]; value != nil && value != false && value != "false" && value != "" {
			return empty, a, nil, fmt.Errorf("unsupported: credential requires native routing")
		}
	}
	delete(data, "disabled")
	stamp := fingerprint(struct {
		ID, Index, Name, Provider, Base, Created string
		Data                                     map[string]any
	}{id, index, a.Name, provider, a.BaseURL, a.CreatedAt.Format(time.RFC3339Nano), data})
	return CredentialSnapshot{AuthID: id, AuthIndex: index, Name: a.Name, Provider: provider, Fingerprint: stamp, UpdatedAt: a.UpdatedAt, FileModifiedAt: a.ModTime, Disabled: a.Disabled || a.Status == "disabled"}, a, data, nil
}

func (h *NativeHost) CredentialSnapshot(ctx context.Context, id, index string) (CredentialSnapshot, error) {
	s, _, _, err := h.credentialProfile(ctx, id, index)
	return s, err
}

func (h *NativeHost) SetCredentialDisabled(ctx context.Context, expected CredentialSnapshot, disabled bool) (CredentialSnapshot, error) {
	current, err := h.CredentialSnapshot(ctx, expected.AuthID, expected.AuthIndex)
	if err != nil || !sameSnapshot(current, expected) || current.Disabled == disabled {
		return CredentialSnapshot{}, fmt.Errorf("conflict: credential changed before status update")
	}
	var response struct {
		Status   string `json:"status"`
		Disabled *bool  `json:"disabled"`
	}
	err = h.Management.request(ctx, http.MethodPatch, "/v8/management/credentials/status", map[string]any{"name": expected.Name, "auth_index": expected.AuthIndex, "disabled": disabled}, &response, 4096)
	if err != nil {
		return CredentialSnapshot{}, err
	}
	if response.Status != "ok" || response.Disabled == nil || *response.Disabled != disabled {
		return CredentialSnapshot{}, fmt.Errorf("management_uncertain: status was not confirmed")
	}
	current, err = h.CredentialSnapshot(ctx, expected.AuthID, expected.AuthIndex)
	if err != nil || !sameCredential(current, expected) || current.Disabled != disabled {
		return CredentialSnapshot{}, fmt.Errorf("management_uncertain: updated credential could not be verified")
	}
	// CPA publishes the persisted file through its watcher after status returns.
	// Wait briefly for this runtime timestamp to settle, while refusing any new
	// file write or content change. Never relax later recovery comparisons.
	fileTime := current.FileModifiedAt
	stable := 0
	for range 8 {
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return CredentialSnapshot{}, ctx.Err()
		case <-timer.C:
		}
		next, readErr := h.CredentialSnapshot(ctx, expected.AuthID, expected.AuthIndex)
		if readErr != nil || !sameCredential(next, expected) || next.Disabled != disabled || !next.FileModifiedAt.Equal(fileTime) {
			return CredentialSnapshot{}, fmt.Errorf("management_uncertain: credential changed during CPA synchronization")
		}
		if sameSnapshot(next, current) {
			stable++
		} else {
			stable = 0
		}
		current = next
		if stable >= 3 {
			return current, nil
		}
	}
	return CredentialSnapshot{}, fmt.Errorf("management_uncertain: CPA credential synchronization did not settle")
}

func (h *NativeHost) DeleteCredential(ctx context.Context, expected CredentialSnapshot) error {
	current, err := h.CredentialSnapshot(ctx, expected.AuthID, expected.AuthIndex)
	if err != nil || !sameSnapshot(current, expected) {
		return fmt.Errorf("conflict: credential changed before removal")
	}
	// Never use the bulk/all endpoint. CPA's delete API is filename based.
	var response struct {
		Status string `json:"status"`
	}
	err = h.Management.request(ctx, http.MethodDelete, "/v8/management/credentials?name="+url.QueryEscape(expected.Name), nil, &response, 4096)
	if err != nil {
		return err
	}
	if response.Status != "ok" {
		return fmt.Errorf("management_uncertain: removal was not confirmed")
	}
	dir, err := h.credentialDirectory(ctx)
	if err != nil || !dir.absent(expected) || !h.missingCredentialFile(ctx, expected.Name) {
		return fmt.Errorf("management_uncertain: removal could not be verified")
	}
	return nil
}

func (h *NativeHost) executeManaged(ctx context.Context, e Execution) (s Sample) {
	start := time.Now()
	s = Sample{Model: e.Model, ExecutionStatus: "failed", QualityVerdict: "unknown", Evidence: Evidence{Path: "cpa_management_probe", Reason: "management_probe_not_business_payload"}}
	defer func() {
		s.TotalMS = time.Since(start).Milliseconds()
		if ctx.Err() != nil {
			s.ExecutionStatus = "cancelled"
			s.Error = "cancelled"
		}
	}()
	expected := *e.ManagedProbe
	current, runtime, data, err := h.credentialProfile(ctx, expected.AuthID, expected.AuthIndex)
	if err != nil || !sameSnapshot(current, expected) || e.AuthID != expected.AuthID || e.Provider != expected.Provider {
		s.Error = "credential changed before management probe"
		return
	}
	base := runtime.BaseURL
	if base == "" {
		base, _ = data["base_url"].(string)
	}
	codex := expected.Provider == "codex"
	if base == "" {
		if codex {
			base = "https://chatgpt.com/backend-api/codex"
		} else {
			base = "https://api.openai.com/v1"
		}
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") {
		s.Error = "invalid credential upstream URL"
		return
	}
	doc := ojson.NewDocString(`{}`)
	fields := map[string]any{"model": e.Model, "stream": false, "max_tokens": e.Parameters.MaxTokens, "messages": []map[string]string{{"role": "user", "content": e.Prompt}}}
	headers := map[string]string{"Authorization": "Bearer $TOKEN$", "Content-Type": "application/json", "Accept": "application/json"}
	path := "/chat/completions"
	if codex {
		path = "/responses"
		fields = map[string]any{"model": e.Model, "stream": true, "store": false, "instructions": "Answer the question directly.", "input": []map[string]any{{"role": "user", "content": []map[string]string{{"type": "input_text", "text": e.Prompt}}}}, "max_output_tokens": e.Parameters.MaxTokens}
		headers["Accept"] = "text/event-stream"
		headers["Originator"] = "codex-tui"
		headers["User-Agent"] = "codex-tui/0.154.0"
		if account, _ := data["account_id"].(string); account != "" {
			headers["Chatgpt-Account-Id"] = account
		}
		if e.Parameters.ReasoningEffort != "" {
			fields["reasoning"] = map[string]string{"effort": e.Parameters.ReasoningEffort}
		}
	} else {
		if e.Parameters.ReasoningEffort != "" {
			fields["reasoning_effort"] = e.Parameters.ReasoningEffort
		}
		if e.Parameters.Temperature != nil {
			fields["temperature"] = *e.Parameters.Temperature
		}
	}
	for k, v := range fields {
		if err := doc.Set(k, v); err != nil {
			s.Error = "request construction failed"
			return
		}
	}
	limit := e.ResponseLimit
	if limit < 1024 {
		limit = h.MaxResponseBytes
	}
	if limit < 1024 {
		limit = 262144
	}
	var response struct {
		StatusCode int    `json:"status_code"`
		Body       string `json:"body"`
	}
	err = h.Management.request(ctx, http.MethodPost, "/v8/management/requests/api-call", map[string]any{"auth_index": expected.AuthIndex, "method": "POST", "url": strings.TrimRight(base, "/") + path, "header": headers, "data": doc.String()}, &response, int64(limit)*6+16384)
	if err != nil {
		s.Error = scrubError(err)
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		s.Error = fmt.Sprintf("upstream HTTP %d", response.StatusCode)
		return
	}
	if len(response.Body) > limit {
		s.Error = "response limit exceeded"
		return
	}
	if codex {
		s.Output, s.ResponseModel, s.Usage, err = parseManagedResponses(response.Body)
	} else {
		c := newCollector(limit)
		err = c.Completion([]byte(response.Body))
		if err == nil && (c.refusal || c.finish != "stop") {
			err = fmt.Errorf("incomplete or refused answer")
		}
		s.Output, s.ResponseModel, s.Usage = c.text.String(), c.model, c.usage
	}
	if err != nil {
		s.Error = scrubError(err)
		return
	}
	if strings.TrimSpace(s.Output) == "" || s.ResponseModel == "" {
		s.ExecutionStatus = "incomplete"
		return
	}
	current, err = h.CredentialSnapshot(ctx, expected.AuthID, expected.AuthIndex)
	if err != nil || !sameSnapshot(current, expected) {
		s.Error = "credential changed during management probe"
		s.Evidence.ConditionsChanged = true
		return
	}
	s.ExecutionStatus = "completed"
	s.Evidence.AuthID = e.AuthID
	s.Evidence.RequestedModel = e.Model
	s.Evidence.ResolvedModel = s.ResponseModel
	s.Evidence.PayloadFingerprint = fingerprint(fields)
	return
}

// Only the complete terminal Responses object is evidence; deltas, truncation,
// refusals, tool calls and successful HTTP statuses alone cannot pass a round.
func parseManagedResponses(body string) (string, string, *Usage, error) {
	var terminal ojson.Result
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			return "", "", nil, fmt.Errorf("invalid responses stream")
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if raw == "[DONE]" {
			continue
		}
		if !ojson.Valid(raw) {
			return "", "", nil, fmt.Errorf("invalid responses event")
		}
		event := ojson.Parse(raw)
		switch event.Get("type").String() {
		case "error", "response.failed", "response.incomplete":
			return "", "", nil, fmt.Errorf("incomplete responses result")
		case "response.completed":
			if terminal.Exists() {
				return "", "", nil, fmt.Errorf("duplicate responses completion")
			}
			terminal = event.Get("response")
		default:
			if terminal.Exists() {
				return "", "", nil, fmt.Errorf("event after responses completion")
			}
		}
	}
	if terminal.Get("status").String() != "completed" || terminal.Get("error").Raw != "" && terminal.Get("error").Raw != "null" {
		return "", "", nil, fmt.Errorf("missing completed responses result")
	}
	var text strings.Builder
	for _, item := range terminal.Get("output").Array() {
		if item.Get("type").String() == "reasoning" {
			continue
		}
		if item.Get("type").String() != "message" || item.Get("status").String() != "completed" {
			return "", "", nil, fmt.Errorf("non-answer responses output")
		}
		for _, part := range item.Get("content").Array() {
			if part.Get("type").String() != "output_text" {
				return "", "", nil, fmt.Errorf("refused or non-text answer")
			}
			text.WriteString(part.Get("text").String())
		}
	}
	u := terminal.Get("usage")
	doc := ojson.NewDocString(`{}`)
	_ = doc.Set("prompt_tokens", u.Get("input_tokens").Value())
	_ = doc.Set("completion_tokens", u.Get("output_tokens").Value())
	_ = doc.Set("total_tokens", u.Get("total_tokens").Value())
	return text.String(), terminal.Get("model").String(), parseUsage(ojson.ParseBytes(doc.Bytes())), nil
}
