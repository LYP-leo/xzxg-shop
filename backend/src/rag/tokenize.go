package rag

import (
	"strings"
	"unicode"
)

func QueryTerms(query string) []string {
	terms := make([]string, 0)
	seen := make(map[string]bool)
	add := func(term string) {
		term = strings.TrimSpace(strings.ToLower(term))
		if len([]rune(term)) < 2 || seen[term] {
			return
		}
		seen[term] = true
		terms = append(terms, term)
	}

	var current []rune
	currentIsHan := false
	flush := func() {
		if len(current) == 0 {
			return
		}
		if currentIsHan {
			addHanNgrams(current, add)
		} else {
			add(string(current))
		}
		current = nil
	}

	for _, r := range query {
		isHan := unicode.In(r, unicode.Han)
		isWord := isHan || unicode.IsLetter(r) || unicode.IsDigit(r)
		if !isWord {
			flush()
			currentIsHan = false
			continue
		}
		if len(current) > 0 && isHan != currentIsHan {
			flush()
		}
		currentIsHan = isHan
		current = append(current, r)
	}
	flush()

	return terms
}

func addHanNgrams(runes []rune, add func(string)) {
	if len(runes) <= 4 {
		add(string(runes))
	}
	for size := 2; size <= 4; size++ {
		if len(runes) < size {
			continue
		}
		for i := 0; i+size <= len(runes); i++ {
			add(string(runes[i : i+size]))
		}
	}
}

func ContainsAnyTerm(text string, terms []string) bool {
	text = strings.ToLower(text)
	for _, term := range terms {
		if strings.Contains(text, strings.ToLower(term)) {
			return true
		}
	}
	return false
}
