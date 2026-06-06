package agent

import (
	"encoding/json"
	"strings"
)

const traceSchemaVersion = "2026-06-05.v1"

func traceMetadata(stage string, eventType string, model string, status string, durationMS int64, metadata map[string]any) map[string]any {
	if metadata == nil {
		metadata = map[string]any{}
	}
	payload := copyTraceMap(metadata)
	if _, ok := payload["schema_version"]; !ok {
		payload["schema_version"] = traceSchemaVersion
	}
	if _, ok := payload["event_kind"]; !ok {
		payload["event_kind"] = traceEventKind(stage, eventType)
	}
	if _, ok := payload["summary"]; !ok {
		payload["summary"] = traceSummaryPayload(stage, eventType, model, status, durationMS, metadata)
	}
	if _, ok := payload["input"]; !ok {
		if input := traceInputPayload(eventType, metadata); len(input) > 0 {
			payload["input"] = input
		}
	}
	if _, ok := payload["output"]; !ok {
		if output := traceOutputPayload(eventType, metadata); len(output) > 0 {
			payload["output"] = output
		}
	}
	if _, ok := payload["debug"]; !ok {
		if debug := traceDebugPayload(metadata); len(debug) > 0 {
			payload["debug"] = debug
		}
	}
	return payload
}

func copyTraceMap(metadata map[string]any) map[string]any {
	out := make(map[string]any, len(metadata)+4)
	for key, value := range metadata {
		out[key] = value
	}
	return out
}

func traceEventKind(stage string, eventType string) string {
	switch eventType {
	case "llm_call":
		return "llm_call"
	}
	if strings.HasPrefix(stage, "tools") {
		return "tool_call"
	}
	if strings.HasPrefix(stage, "skills") {
		return "skill_call"
	}
	if strings.HasPrefix(stage, "planner") {
		return "planner"
	}
	if strings.HasPrefix(stage, "memory") {
		return "memory"
	}
	if strings.HasPrefix(stage, "risk") {
		return "risk_check"
	}
	if strings.HasPrefix(stage, "react.final_format") {
		return "final_output"
	}
	return eventType
}

func traceSummaryPayload(stage string, eventType string, model string, status string, durationMS int64, metadata map[string]any) map[string]any {
	summary := map[string]any{
		"stage":       stage,
		"event_type":  eventType,
		"status":      status,
		"duration_ms": durationMS,
	}
	if model != "" {
		summary["model"] = model
	}
	for _, key := range []string{"route", "intent", "protocol", "ok", "message", "relevance_status", "relevance_reason", "reason", "code"} {
		if value, ok := metadata[key]; ok {
			summary[key] = value
		}
	}
	if ids := stringSliceFromAny(metadata["product_ids"]); len(ids) > 0 {
		summary["product_ids"] = ids
	}
	if ids := stringSliceFromAny(metadata["candidate_product_ids"]); len(ids) > 0 {
		summary["candidate_product_ids"] = ids
	}
	if ids := stringSliceFromAny(metadata["dropped_product_ids"]); len(ids) > 0 {
		summary["dropped_product_ids"] = ids
	}
	if ids := stringSliceFromAny(metadata["chunk_ids"]); len(ids) > 0 {
		summary["chunk_ids"] = ids
	}
	if count, ok := metadata["result_item_count"]; ok {
		summary["result_item_count"] = count
	}
	return summary
}

func traceInputPayload(eventType string, metadata map[string]any) map[string]any {
	input := map[string]any{}
	if eventType == "llm_call" {
		for _, key := range []string{"messages", "temperature", "prompt_chars", "prompt_hash"} {
			if value, ok := metadata[key]; ok {
				input[key] = value
			}
		}
		return input
	}
	if value, ok := metadata["arguments"]; ok {
		input["arguments"] = normalizeTraceJSONValue(value)
	}
	if query, ok := metadata["query"]; ok {
		input["query"] = query
	}
	if value, ok := metadata["original_query"]; ok {
		input["original_query"] = value
	}
	if value, ok := metadata["normalized_query"]; ok {
		input["normalized_query"] = value
	}
	return input
}

func traceOutputPayload(eventType string, metadata map[string]any) map[string]any {
	output := map[string]any{}
	if eventType == "llm_call" {
		for _, key := range []string{"raw_output", "filtered_output", "raw_length", "streamed", "final_found"} {
			if value, ok := metadata[key]; ok {
				output[key] = value
			}
		}
		return output
	}
	for _, key := range []string{"result", "observation", "observation_json", "blocks", "blocks_json", "filtered_output", "raw_output"} {
		if value, ok := metadata[key]; ok {
			output[key] = normalizeTraceJSONValue(value)
		}
	}
	return output
}

func traceDebugPayload(metadata map[string]any) map[string]any {
	debug := map[string]any{}
	for _, key := range []string{
		"used_record_ids",
		"referenced_product_ids",
		"memory_summary",
		"candidate_count",
		"attachment_count",
		"deterministic",
		"result_truncated",
		"admin_visible_note",
	} {
		if value, ok := metadata[key]; ok {
			debug[key] = value
		}
	}
	return debug
}

func normalizeTraceJSONValue(value any) any {
	switch typed := value.(type) {
	case json.RawMessage:
		var parsed any
		if err := json.Unmarshal(typed, &parsed); err == nil {
			return parsed
		}
		return string(typed)
	case []byte:
		var parsed any
		if err := json.Unmarshal(typed, &parsed); err == nil {
			return parsed
		}
		return string(typed)
	default:
		return value
	}
}

func stringSliceFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(anyToString(item)); text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func anyToString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		bytes, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(bytes)
	}
}
