package upstream

import (
	"encoding/json"
	"log"
)

func injectCacheControl(body []byte) []byte {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return body
	}

	injectSystem(payload)
	injectMessages(payload)

	result, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return result
}

func injectSystem(payload map[string]any) {
	systemVal, ok := payload["system"]
	if !ok || systemVal == nil {
		return
	}

	marker := map[string]any{"type": "ephemeral"}

	switch s := systemVal.(type) {
	case string:
		payload["system"] = []any{
			map[string]any{
				"type":          "text",
				"text":          s,
				"cache_control": marker,
			},
		}
	case []any:
		if len(s) == 0 {
			return
		}
		if block, ok := s[len(s)-1].(map[string]any); ok {
			if _, exists := block["cache_control"]; !exists {
				block["cache_control"] = marker
			}
		}
	}
}

func injectMessages(payload map[string]any) {
	msgsVal, ok := payload["messages"]
	if !ok {
		return
	}
	msgs, ok := msgsVal.([]any)
	if !ok || len(msgs) == 0 {
		return
	}

	marker := map[string]any{"type": "ephemeral"}

	// Pick target: second-to-last message if >= 2, otherwise the only message
	targetIdx := len(msgs) - 2
	if targetIdx < 0 {
		targetIdx = 0
	}

	msg, ok := msgs[targetIdx].(map[string]any)
	if !ok {
		return
	}

	contentVal, ok := msg["content"]
	if !ok {
		return
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
	case []any:
		if len(c) == 0 {
			return
		}
		if block, ok := c[len(c)-1].(map[string]any); ok {
			if _, exists := block["cache_control"]; !exists {
				block["cache_control"] = marker
			}
		}
	}
}

func logCacheInjection(body []byte) {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[cache-debug] JSON parse failed")
		return
	}

	hasSystem := false
	systemType := "none"
	if s, ok := payload["system"]; ok && s != nil {
		hasSystem = true
		switch sv := s.(type) {
		case string:
			systemType = "string"
		case []any:
			systemType = "array"
			if len(sv) > 0 {
				if block, ok := sv[len(sv)-1].(map[string]any); ok {
					if _, ok := block["cache_control"]; ok {
						systemType = "array+cc"
					}
				}
			}
		default:
			systemType = "unknown"
		}
	}

	msgCount := 0
	msgCC := "none"
	if m, ok := payload["messages"]; ok {
		if msgs, ok := m.([]any); ok {
			msgCount = len(msgs)
			targetIdx := len(msgs) - 2
			if targetIdx < 0 {
				targetIdx = 0
			}
			if msgCount > 0 {
				if msg, ok := msgs[targetIdx].(map[string]any); ok {
					if c, ok := msg["content"]; ok {
						switch cv := c.(type) {
						case string:
							msgCC = "string(no-cc)"
						case []any:
							if len(cv) > 0 {
								if block, ok := cv[len(cv)-1].(map[string]any); ok {
									if _, ok := block["cache_control"]; ok {
										msgCC = "array+cc"
									} else {
										msgCC = "array(no-cc)"
									}
								}
							}
						default:
							_ = cv
						}
					}
				}
			}
		}
	}

	toolCount := 0
	if t, ok := payload["tools"]; ok {
		if tools, ok := t.([]any); ok {
			toolCount = len(tools)
		}
	}

	model, _ := payload["model"].(string)

	log.Printf("[cache-debug] model=%s system=%v(%s) msgs=%d tools=%d msg_target=%s",
		model, hasSystem, systemType, msgCount, toolCount, msgCC)
}
