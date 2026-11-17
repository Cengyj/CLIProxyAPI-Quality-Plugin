package quality

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/MoeForge/moego/ojson"
)

type collector struct {
	pending       []byte
	text          strings.Builder
	limit         int
	received      int
	done          bool
	rawEvents     bool
	finish, model string
	refusal       bool
	usage         *Usage
}

func newCollector(limit int) *collector { return &collector{limit: limit} }
func (c *collector) Feed(raw []byte) error {
	c.received += len(raw)
	if c.received > c.limit {
		return fmt.Errorf("response limit exceeded")
	}
	c.pending = append(c.pending, raw...)
	for {
		pos := bytes.IndexByte(c.pending, '\n')
		if pos < 0 {
			break
		}
		line := strings.TrimSpace(string(c.pending[:pos]))
		c.pending = c.pending[pos+1:]
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "id:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			return fmt.Errorf("invalid SSE line")
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			c.done = true
			continue
		}
		if c.done {
			return fmt.Errorf("data after terminal event")
		}
		if err := c.event([]byte(data), true); err != nil {
			return err
		}
	}
	return nil
}
func (c *collector) End() error {
	if len(bytes.TrimSpace(c.pending)) > 0 {
		return fmt.Errorf("incomplete SSE frame")
	}
	if !c.done || c.finish == "" {
		return fmt.Errorf("premature stream EOF")
	}
	return nil
}
func (c *collector) Completion(raw []byte) error {
	if err := c.event(raw, false); err != nil {
		return err
	}
	c.done = true
	return nil
}
func (c *collector) event(raw []byte, stream bool) error {
	if !ojson.ValidBytes(raw) {
		return fmt.Errorf("invalid upstream JSON")
	}
	root := ojson.ParseBytes(raw)
	if root.Get("error").Exists() {
		return fmt.Errorf("upstream error event")
	}
	if model := root.Get("model").String(); model != "" {
		c.model = model
	}
	u := root.Get("usage")
	if u.Exists() && u.Raw != "null" {
		c.usage = parseUsage(u)
	}
	choices := root.Get("choices").Array()
	if len(choices) > 1 {
		return fmt.Errorf("unexpected multiple choices")
	}
	if len(choices) == 0 {
		if stream && u.Exists() {
			return nil
		}
		return fmt.Errorf("no completion choices")
	}
	choice := choices[0]
	key := "message"
	if stream {
		key = "delta"
	}
	msg := choice.Get(key)
	if msg.Get("refusal").String() != "" {
		c.refusal = true
	}
	if msg.Get("tool_calls").Exists() {
		return fmt.Errorf("tool output is not a completed answer")
	}
	content := msg.Get("content")
	if content.Exists() && content.Raw != "null" {
		if content.Type != ojson.String {
			return fmt.Errorf("non-text content")
		}
		if c.finish != "" && content.String() != "" {
			return fmt.Errorf("answer content after completion status")
		}
		if c.text.Len()+len(content.String()) > c.limit {
			return fmt.Errorf("output limit exceeded")
		}
		c.text.WriteString(content.String())
	}
	finish := choice.Get("finish_reason").String()
	if finish != "" {
		if c.finish != "" && c.finish != finish {
			return fmt.Errorf("conflicting completion status")
		}
		c.finish = finish
	}
	return nil
}

func parseUsage(value ojson.Result) *Usage {
	counts := [3]int64{}
	for i, key := range []string{"prompt_tokens", "completion_tokens", "total_tokens"} {
		field := value.Get(key)
		if field.Type != ojson.Number {
			return nil
		}
		count, err := strconv.ParseInt(field.Raw, 10, 64)
		if err != nil || count < 0 {
			return nil
		}
		counts[i] = count
	}
	if counts[2] < counts[0] || counts[2]-counts[0] < counts[1] {
		return nil
	}
	return &Usage{Input: counts[0], Output: counts[1], Total: counts[2]}
}

// FeedHostChunk also accepts complete event records emitted without HTTP SSE delimiters by the host translator.
func (c *collector) FeedHostChunk(raw []byte) error {
	if len(c.pending) == 0 && ojson.ValidBytes(raw) {
		if c.done {
			return fmt.Errorf("data after terminal event")
		}
		c.rawEvents = true
		c.received += len(raw)
		if c.received > c.limit {
			return fmt.Errorf("response limit exceeded")
		}
		return c.event(raw, true)
	}
	combined := append(append([]byte(nil), c.pending...), raw...)
	if !bytes.ContainsRune(combined, '\n') {
		line := strings.TrimSpace(string(combined))
		// Official executors can forward scanner lines and append a bare
		// terminal sentinel. Preserve those record boundaries before buffering.
		if len(c.pending) == 0 && (strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "id:")) {
			return c.Feed(append(combined, '\n', '\n'))
		}
		if line == "[DONE]" {
			c.received -= len(c.pending)
			c.pending = nil
			return c.Feed([]byte("data: [DONE]\n\n"))
		}
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if payload == "[DONE]" || ojson.Valid(payload) {
				c.received -= len(c.pending)
				c.pending = nil
				return c.Feed(append(combined, '\n', '\n'))
			}
		}
	}
	return c.Feed(raw)
}
