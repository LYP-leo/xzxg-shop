package risk

import (
	"encoding/json"
	"regexp"
	"strings"
)

type Rule struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Words   []string `json:"words"`
	Pattern string   `json:"pattern"`
}

type Result struct {
	Blocked bool   `json:"blocked"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Matched string `json:"matched,omitempty"`
}

func CheckText(text string, values map[string]string) Result {
	if !configBool(values, "risk.enabled", true) {
		return Result{}
	}
	rules := parseRules(values["risk.rules_json"])
	lower := strings.ToLower(text)
	for _, rule := range rules {
		for _, word := range rule.Words {
			word = strings.TrimSpace(word)
			if word == "" {
				continue
			}
			if strings.Contains(lower, strings.ToLower(word)) {
				return blocked(rule, word)
			}
		}
		if strings.TrimSpace(rule.Pattern) == "" {
			continue
		}
		compiled, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}
		if match := compiled.FindString(text); match != "" {
			return blocked(rule, match)
		}
	}
	return Result{}
}

func parseRules(raw string) []Rule {
	var rules []Rule
	if err := json.Unmarshal([]byte(raw), &rules); err == nil && len(rules) > 0 {
		return rules
	}
	return []Rule{
		{Code: "unsafe_request", Message: "这个请求涉及平台风控限制，无法继续处理。", Words: []string{"违法", "违禁", "假货", "绕过风控"}},
	}
}

func blocked(rule Rule, matched string) Result {
	message := strings.TrimSpace(rule.Message)
	if message == "" {
		message = "这个请求涉及平台风控限制，无法继续处理。"
	}
	code := strings.TrimSpace(rule.Code)
	if code == "" {
		code = "risk_blocked"
	}
	return Result{Blocked: true, Code: code, Message: message, Matched: matched}
}

func configBool(values map[string]string, key string, fallback bool) bool {
	value := strings.TrimSpace(values[key])
	if value == "" {
		return fallback
	}
	return value == "true" || value == "1" || value == "yes"
}
