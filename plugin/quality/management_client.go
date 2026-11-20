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
	"slices"
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
	return m.conditionalResponse(ctx, method, path, body, "")
}

func (m *ManagementClient) conditionalResponse(ctx context.Context, method, path string, body any, ifMatch string) (*http.Response, error) {
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
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
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
	ProviderGroupID   string   `json:"provider_group_id,omitempty"`
	ProviderMechanism string   `json:"provider_mechanism,omitempty"`
	ProviderMembers   []string `json:"provider_members,omitempty"`
	ProviderETag      string   `json:"provider_etag,omitempty"`
	// The restore operation can put an inherited field back in its original form.
	ProviderExclusionSource string    `json:"provider_exclusion_source,omitempty"`
	AccountFingerprint      string    `json:"account_fingerprint,omitempty"`
	AuthID                  string    `json:"auth_id"`
	AuthIndex               string    `json:"auth_index"`
	Name                    string    `json:"name"`
	Provider                string    `json:"provider"`
	Fingerprint             string    `json:"fingerprint"`
	UpdatedAt               time.Time `json:"updated_at"`
	FileModifiedAt          time.Time `json:"file_modified_at"`
	Disabled                bool      `json:"disabled"`
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
	if a.ProviderMechanism != "" || b.ProviderMechanism != "" {
		return sameCredential(a, b) && a.Disabled == b.Disabled && a.ProviderMechanism == b.ProviderMechanism &&
			a.ProviderGroupID != "" && a.ProviderGroupID == b.ProviderGroupID && slices.Equal(a.ProviderMembers, b.ProviderMembers)
	}
	return sameCredential(a, b) && a.Disabled == b.Disabled && !a.UpdatedAt.IsZero() && a.UpdatedAt.Equal(b.UpdatedAt) && a.FileModifiedAt.Equal(b.FileModifiedAt)
}

func (h *NativeHost) credentialProfile(ctx context.Context, id, index string) (CredentialSnapshot, pluginapi.HostAuthFileEntry, map[string]any, error) {
	var runtime pluginapi.HostAuthGetRuntimeResponse
	var raw pluginapi.HostAuthGetResponse
	var empty CredentialSnapshot
	if id == "" || index == "" {
		return empty, runtime.Auth, nil, rejectScope("blocked: credential identity is missing", "identity_missing", "凭证标识或索引缺失，无法读取快照。")
	}
	if err := h.call(ctx, "host.auth.get_runtime", pluginapi.HostAuthGetRequest{AuthIndex: index}, &runtime); err != nil {
		return empty, runtime.Auth, nil, rejectScope("blocked: credential no longer exists", "snapshot_unavailable", "CPA 未能按索引返回该凭证的运行信息。")
	}
	a := runtime.Auth
	provider := a.Provider
	if provider == "" {
		provider = a.Type
	}
	const unsupportedFile = "unsupported: automatic handling requires one identifiable file credential"
	if a.ID != id || a.AuthIndex != index {
		return empty, a, nil, rejectScope(unsupportedFile, "identity_conflict", "CPA 返回的凭证标识或索引与本轮目标不一致。")
	}
	if providerFamily(a.ID, provider) != "" {
		return empty, a, nil, rejectScope(unsupportedFile, "provider_route", "供应商配置通过 CPA 原生路由检测，不能作为文件凭证处置。")
	}
	if a.RuntimeOnly || a.Source != "file" || a.Name == "" || filepath.Base(a.Name) != a.Name || strings.ContainsAny(a.Name, "/\\") || !strings.HasSuffix(strings.ToLower(a.Name), ".json") {
		return empty, a, nil, rejectScope(unsupportedFile, "unsupported_file", "管理探测需要可单独标识的 JSON 文件凭证。")
	}
	if a.UpdatedAt.IsZero() {
		return empty, a, nil, rejectScope(unsupportedFile, "snapshot_unavailable", "CPA 缺少凭证的运行更新时间，无法核对快照。")
	}
	if provider != "codex" && provider != "openai" && provider != "openai-compatibility" {
		return empty, a, nil, rejectScope("unsupported: management probes currently support OpenAI and Codex file credentials", "unsupported_provider", "当前提供方不支持凭证管理探测。")
	}
	if err := h.call(ctx, "host.auth.get", pluginapi.HostAuthGetRequest{AuthIndex: index}, &raw); err != nil {
		return empty, a, nil, rejectScope("blocked: credential snapshot is unavailable", "snapshot_unavailable", "CPA 未能返回该凭证的文件快照。")
	}
	var data map[string]any
	if raw.AuthIndex != index || raw.Name != a.Name {
		return empty, a, nil, rejectScope("blocked: credential snapshot identity changed", "identity_conflict", "CPA 返回的文件名或索引与运行信息不一致。")
	}
	if json.Unmarshal(raw.JSON, &data) != nil || data == nil {
		return empty, a, nil, rejectScope("blocked: credential snapshot identity changed", "snapshot_unavailable", "无法解析凭证文件快照。")
	}
	// Expanded sources, SIWC, and ambiguous routing need their native executor.
	for _, field := range []string{"use_siwc", "virtual", "projects", "project_ids"} {
		if value := data[field]; value != nil && value != false && value != "false" && value != "" {
			return empty, a, nil, rejectScope("unsupported: credential requires native routing", "native_routing_required", "该凭证需要使用宿主原生路由。")
		}
	}
	fileDisabled, hasDisabled := data["disabled"]
	delete(data, "disabled")
	stamp := fingerprint(struct {
		ID, Index, Name, Provider, Base, Created string
		Data                                     map[string]any
	}{id, index, a.Name, provider, a.BaseURL, a.CreatedAt.Format(time.RFC3339Nano), data})
	if hasDisabled {
		data["disabled"] = fileDisabled
	}
	snapshot := CredentialSnapshot{AuthID: id, AuthIndex: index, Name: a.Name, Provider: provider, Fingerprint: stamp, UpdatedAt: a.UpdatedAt, FileModifiedAt: a.ModTime, Disabled: a.Disabled || a.Status == "disabled"}
	if account, _ := data["account_id"].(string); strings.TrimSpace(account) != "" {
		snapshot.AccountFingerprint = fingerprint([2]string{provider, account})
	}
	return snapshot, a, data, nil
}

// Re-read the pinned runtime and physical file through public SDK callbacks.
// Token refresh and host restart may both change the full snapshot fingerprint.
func (h *NativeHost) verifyEnabledManualFile(ctx context.Context, t Target, d *CredentialDisposition) (string, error) {
	current, runtime, data, err := h.credentialProfile(ctx, t.AuthID, t.AuthIndex)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("blocked: 无法核实 CPA 当前凭证文件，请刷新后重试。")
	}
	if current.Disabled || runtime.Status != "active" || runtime.Unavailable || runtime.NextRetryAfter.After(time.Now()) {
		return "", fmt.Errorf("blocked: CPA 当前凭证已停用或暂不可用。")
	}
	if disabled, exists := data["disabled"]; exists && disabled != false {
		return "", fmt.Errorf("blocked: CPA 文件尚未确认启用此凭证，请核对文件状态。")
	}
	old := d.Snapshot
	fileType, _ := data["type"].(string)
	fileEmail, _ := data["email"].(string)
	email := strings.TrimSpace(d.Email)
	sameEmail := func(value string) bool { return strings.EqualFold(email, strings.TrimSpace(value)) }
	if current.AuthID != old.AuthID || current.AuthIndex != old.AuthIndex || current.Provider != old.Provider || current.Name != old.Name ||
		fileType != current.Provider || old.Fingerprint == "" ||
		email != "" && (t.Email != "" && !sameEmail(t.Email) || runtime.Email != "" && !sameEmail(runtime.Email) || fileEmail != "" && !sameEmail(fileEmail)) {
		return "", fmt.Errorf("blocked: CPA 当前凭证与原错误处置记录的身份不一致，请核对账号。")
	}
	if old.AccountFingerprint != "" {
		if old.AccountFingerprint != current.AccountFingerprint {
			return "", fmt.Errorf("blocked: CPA 当前账号标识与原错误处置记录不一致，请核对账号。")
		}
	} else if current.Fingerprint != old.Fingerprint && (email == "" || !sameEmail(fileEmail)) {
		return "", fmt.Errorf("blocked: 无法确认刷新后的凭证仍属于原账号，请在质量运维中核对。")
	}
	// Pin the selected account throughout this run, including for legacy
	// dispositions whose only retained principal was an email address.
	if current.AccountFingerprint != "" {
		return current.AccountFingerprint, nil
	}
	if email != "" && sameEmail(fileEmail) {
		return fingerprint([2]string{current.Provider, strings.ToLower(email)}), nil
	}
	return current.Fingerprint, nil
}

func (h *NativeHost) CredentialSnapshot(ctx context.Context, id, index string) (CredentialSnapshot, error) {
	if familyOfProviderID(id) != "" {
		route, err := h.pinnedConfiguredRoute(ctx, id, index)
		if err != nil {
			return CredentialSnapshot{}, err
		}
		return route.snapshot(), nil
	}
	s, _, _, err := h.credentialProfile(ctx, id, index)
	return s, err
}

func (h *NativeHost) SetCredentialDisabled(ctx context.Context, expected CredentialSnapshot, disabled bool) (CredentialSnapshot, error) {
	if expected.ProviderMechanism != "" {
		return h.setProviderDisabled(ctx, expected, disabled)
	}
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
	if expected.ProviderMechanism != "" || familyOfProviderID(expected.AuthID) != "" {
		return fmt.Errorf("unsupported: provider configuration cannot be removed")
	}
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
			s.UpstreamCondition = ""
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
	confirmProbe := func() bool {
		after, readErr := h.CredentialSnapshot(ctx, expected.AuthID, expected.AuthIndex)
		if readErr != nil || !sameSnapshot(after, expected) {
			s.Error = "credential changed during management probe"
			s.Evidence.ConditionsChanged = true
			return false
		}
		s.Evidence.AuthID = e.AuthID
		s.Evidence.RequestedModel = e.Model
		s.Evidence.PayloadFingerprint = fingerprint(fields)
		return true
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		s.Error = fmt.Sprintf("upstream HTTP %d", response.StatusCode)
		if len(response.Body) <= limit {
			if condition := managedUpstreamCondition(response.StatusCode, response.Body, e.Model); condition != "" && confirmProbe() {
				s.UpstreamCondition = condition
			}
		}
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
	if !confirmProbe() {
		return
	}
	s.ExecutionStatus = "completed"
	s.Evidence.ResolvedModel = s.ResponseModel
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
	output := terminal.Get("output")
	if !output.IsArray() {
		return "", "", nil, fmt.Errorf("invalid responses output")
	}
	var text strings.Builder
	answerSeen := false
	var completionErr error
	for _, item := range output.Array() {
		if item.Get("type").String() == "reasoning" {
			continue
		}
		if item.Get("type").String() != "message" {
			return "", "", nil, fmt.Errorf("non-answer responses output")
		}
		if item.Get("status").String() != "completed" {
			completionErr = fmt.Errorf("unconfirmed responses message status")
		}
		if role := item.Get("role"); role.Exists() && (role.Type != ojson.String || role.String() != "assistant") {
			return "", "", nil, fmt.Errorf("non-assistant responses message")
		}
		// Older endpoints omit phase. A single unphased message can be the answer.
		phase := item.Get("phase")
		commentary := phase.Type == ojson.String && phase.String() == "commentary"
		if phase.Exists() && phase.Raw != "null" && !commentary && (phase.Type != ojson.String || phase.String() != "final_answer") {
			return "", "", nil, fmt.Errorf("unknown responses message phase")
		}
		if answerSeen {
			return "", "", nil, fmt.Errorf("unexpected message after final answer")
		}
		content := item.Get("content")
		if !content.IsArray() {
			return "", "", nil, fmt.Errorf("invalid responses message content")
		}
		for _, part := range content.Array() {
			value := part.Get("text")
			if part.Get("type").String() != "output_text" || value.Type != ojson.String {
				return "", "", nil, fmt.Errorf("refused or non-text answer")
			}
			if !commentary {
				text.WriteString(value.String())
			}
		}
		answerSeen = !commentary
	}
	if !answerSeen {
		return "", "", nil, fmt.Errorf("missing final responses answer")
	}
	model := terminal.Get("model")
	if model.Exists() && model.Raw != "null" && model.Type != ojson.String {
		return text.String(), "", nil, fmt.Errorf("invalid response model")
	}
	u := terminal.Get("usage")
	doc := ojson.NewDocString(`{}`)
	_ = doc.Set("prompt_tokens", u.Get("input_tokens").Value())
	_ = doc.Set("completion_tokens", u.Get("output_tokens").Value())
	_ = doc.Set("total_tokens", u.Get("total_tokens").Value())
	return text.String(), strings.TrimSpace(model.String()), parseUsage(ojson.ParseBytes(doc.Bytes())), completionErr
}
