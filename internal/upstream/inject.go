package upstream

import "encoding/json"

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
			block["cache_control"] = marker
		}
	}
}

func injectMessages(payload map[string]any) {
	msgsVal, ok := payload["messages"]
	if !ok {
		return
	}
	msgs, ok := msgsVal.([]any)
	if !ok || len(msgs) < 2 {
		return
	}

	marker := map[string]any{"type": "ephemeral"}

	msg, ok := msgs[len(msgs)-2].(map[string]any)
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
			block["cache_control"] = marker
		}
	}
}
