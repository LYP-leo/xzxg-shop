package risk

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/LYP-leo/xzxg-shop/backend/src/domain"
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

func CheckAccount(account domain.Account, values map[string]string) Result {
	if !configBool(values, "risk.enabled", true) {
		return Result{}
	}
	if !isConfiguredRiskStatus(account.Status, values["risk.account_statuses"]) {
		return Result{}
	}
	message := strings.TrimSpace(values["risk.account_message"])
	if message == "" {
		message = "当前账号命中平台风控限制，暂时无法继续使用导购 Agent。"
	}
	return Result{
		Blocked: true,
		Code:    "risk_account",
		Message: message,
		Matched: account.Status,
	}
}

func isConfiguredRiskStatus(status string, raw string) bool {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		return false
	}
	statuses := []string{domain.AccountStatusRisk}
	if strings.TrimSpace(raw) != "" {
		statuses = strings.Split(raw, ",")
	}
	for _, item := range statuses {
		if strings.TrimSpace(strings.ToLower(item)) == status {
			return true
		}
	}
	return false
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
