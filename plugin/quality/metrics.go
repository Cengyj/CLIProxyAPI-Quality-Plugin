package quality

import (
	"fmt"
	"sort"
)

func (en *Engine) Metrics() (map[string]any, error) {
	en.mu.Lock()
	correct, incorrect, unknown, planned := 0, 0, 0, 0
	errors := map[string]int{}
	latencies := []int64{}
	ttfts := []int64{}
	var knownTokens, judgeTokens, controlTokens int64
	judgeUnknown, controlUnknown := 0, 0
	unknownUsage := 0
	for _, r := range en.runs {
		planned += max(0, len(r.Config.Models)*r.Config.Samples-r.PrunedSamples)
		for _, s := range r.Controls {
			if s.Usage != nil {
				controlTokens += s.Usage.Total
			} else {
				controlUnknown++
			}
			if r.Config.Judge.Mode == "llm" {
				if s.JudgeUsage != nil {
					judgeTokens += s.JudgeUsage.Total
				} else {
					judgeUnknown++
				}
			}
		}
		for _, s := range r.Samples {
			if r.Config.Judge.Mode == "llm" {
				if s.JudgeUsage != nil {
					judgeTokens += s.JudgeUsage.Total
				} else {
					judgeUnknown++
				}
			}
			latencies = append(latencies, s.TotalMS)
			if s.TTFTMS != nil {
				ttfts = append(ttfts, *s.TTFTMS)
			}
			switch s.QualityVerdict {
			case "correct":
				correct++
			case "incorrect":
				incorrect++
			default:
				unknown++
			}
			if s.ExecutionStatus != "completed" {
				errors[s.ExecutionStatus]++
			}
			if s.Usage != nil {
				knownTokens += s.Usage.Total
			} else {
				unknownUsage++
			}
		}
	}
	var rate, coverage any
	if correct+incorrect > 0 {
		rate = float64(correct) / float64(correct+incorrect)
	}
	if planned > 0 {
		coverage = float64(correct+incorrect) / float64(planned)
	}
	budget := en.budget
	en.mu.Unlock()
	sources := map[string]map[string]int64{}
	err := en.store.All("usage", func(raw []byte) error {
		var event UsageEvent
		if err := decode(raw, &event); err != nil {
			return err
		}
		source := trafficSource(event.Source)
		bucket := sources[source]
		if bucket == nil {
			bucket = map[string]int64{"attempts": 0, "failed": 0, "known_tokens": 0, "unknown_usage": 0}
			sources[source] = bucket
		}
		bucket["attempts"]++
		if event.Failed {
			bucket["failed"]++
		}
		if event.TokensKnown && event.Tokens >= 0 {
			bucket["known_tokens"] += event.Tokens
		} else {
			bucket["unknown_usage"]++
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("unavailable: read usage metrics: %w", err)
	}
	return map[string]any{"latency_ms": percentiles(latencies), "ttft_ms": percentiles(ttfts), "traffic_sources": sources, "correct": correct, "incorrect": incorrect, "unknown": unknown, "planned": planned, "quality_pass_rate": rate, "coverage": coverage, "known_tokens": knownTokens + judgeTokens + controlTokens, "subject_known_tokens": knownTokens, "judge_known_tokens": judgeTokens, "control_known_tokens": controlTokens, "judge_unknown_usage": judgeUnknown, "control_unknown_usage": controlUnknown, "unknown_usage": unknownUsage, "error_types": errors, "budget": budget, "dropped_usage_events": en.dropped.Load(), "usage_semantics": "execution attempts, not inbound requests"}, nil
}

func percentiles(values []int64) any {
	if len(values) == 0 {
		return nil
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return map[string]any{"count": len(values), "p50": values[(len(values)-1)/2], "p95": values[(len(values)-1)*95/100]}
}
