package promptcache

import (
	"encoding/json"
	"log"
)

type Options struct {
	TTL string
}

type Report struct {
	Model                string
	ToolCount            int
	HasSystem            bool
	SystemMarked         bool
	SystemAlreadyMarked  bool
	MessageCount         int
	MessageIndex         int
	MessageMarked        bool
	MessageAlreadyMarked bool
	Changed              bool
	SkippedReason        string
}

func Inject(body []byte, opts Options) ([]byte, Report, bool) {
	var report Report
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		report.SkippedReason = "invalid_json"
		return body, report, false
	}

	report.Model, _ = payload["model"].(string)
	if tools, ok := payload["tools"].([]any); ok {
		report.ToolCount = len(tools)
	}

	marker := cacheMarker(opts)
	if marked, already := injectSystem(payload, marker); marked || already {
		report.HasSystem = true
		report.SystemMarked = marked
		report.SystemAlreadyMarked = already
		report.Changed = report.Changed || marked
	} else if _, ok := payload["system"]; ok {
		report.HasSystem = true
	}

	marked, already, count, index, reason := injectMessages(payload, marker)
	report.MessageCount = count
	report.MessageIndex = index
	report.MessageMarked = marked
	report.MessageAlreadyMarked = already
	report.Changed = report.Changed || marked
	if reason != "" && !report.Changed && !report.SystemAlreadyMarked && !report.MessageAlreadyMarked {
		report.SkippedReason = reason
	}

	if !report.Changed {
		return body, report, false
	}
	result, err := json.Marshal(payload)
	if err != nil {
		report.SkippedReason = "marshal_failed"
		return body, report, false
	}
	return result, report, true
}

func LogReport(path string, report Report) {
	log.Printf("[cache-debug] path=%s model=%s system=%v system_cc=%v system_existing=%v msgs=%d msg_idx=%d msg_cc=%v msg_existing=%v tools=%d changed=%v skipped=%s",
		path,
		report.Model,
		report.HasSystem,
		report.SystemMarked,
		report.SystemAlreadyMarked,
		report.MessageCount,
		report.MessageIndex,
		report.MessageMarked,
		report.MessageAlreadyMarked,
		report.ToolCount,
		report.Changed,
		report.SkippedReason,
	)
}

func cacheMarker(opts Options) map[string]any {
	marker := map[string]any{"type": "ephemeral"}
	if opts.TTL == "1h" {
		marker["ttl"] = "1h"
	}
	return marker
}

func injectSystem(payload map[string]any, marker map[string]any) (bool, bool) {
	systemVal, ok := payload["system"]
	if !ok || systemVal == nil {
		return false, false
	}

	switch s := systemVal.(type) {
	case string:
		payload["system"] = []any{
			map[string]any{
				"type":          "text",
				"text":          s,
				"cache_control": marker,
			},
		}
		return true, false
	case []any:
		if len(s) == 0 {
			return false, false
		}
		for i := len(s) - 1; i >= 0; i-- {
			block, ok := s[i].(map[string]any)
			if !ok {
				continue
			}
			return addMarker(block, marker)
		}
	}
	return false, false
}

func injectMessages(payload map[string]any, marker map[string]any) (bool, bool, int, int, string) {
	msgsVal, ok := payload["messages"]
	if !ok {
		return false, false, 0, -1, "no_messages"
	}
	msgs, ok := msgsVal.([]any)
	if !ok || len(msgs) == 0 {
		return false, false, 0, -1, "no_messages"
	}

	// Pick target: second-to-last message if >= 2, otherwise the only message.
	targetIdx := len(msgs) - 2
	if targetIdx < 0 {
		targetIdx = 0
	}

	msg, ok := msgs[targetIdx].(map[string]any)
	if !ok {
		return false, false, len(msgs), targetIdx, "unsupported_message"
	}

	contentVal, ok := msg["content"]
	if !ok {
		return false, false, len(msgs), targetIdx, "no_content"
	}

	switch c := contentVal.(type) {
	case string:
		msg["content"] = []any{
			map[string]any{
				"type":          "text",
				"text":          c,
				"cache_control": marker,
			},
		}
		return true, false, len(msgs), targetIdx, ""
	case []any:
		if len(c) == 0 {
			return false, false, len(msgs), targetIdx, "empty_content"
		}
		for i := len(c) - 1; i >= 0; i-- {
			block, ok := c[i].(map[string]any)
			if !ok {
				continue
			}
			marked, already := addMarker(block, marker)
			return marked, already, len(msgs), targetIdx, ""
		}
	}
	return false, false, len(msgs), targetIdx, "unsupported_content"
}

func addMarker(block map[string]any, marker map[string]any) (bool, bool) {
	if _, exists := block["cache_control"]; exists {
		return false, true
	}
	block["cache_control"] = marker
	return true, false
}
