package pricing

import (
	"fmt"
	"strconv"
	"strings"
)

// Money is CNY minor units. Keep the database DECIMAL(10,2) bound explicit.
type Money int64

const MaxMoney Money = 9_999_999_999

func ParseMoney(value string) (Money, error) {
	n, err := parseDecimal(value, 2)
	if err != nil || n > int64(MaxMoney) {
		return 0, fmt.Errorf("invalid money %q", value)
	}
	return Money(n), nil
}

func ParseRate(value string) (int64, error) {
	n, err := parseDecimal(value, 4)
	if err != nil || n > 10000 {
		return 0, fmt.Errorf("invalid discount fraction %q", value)
	}
	return n, nil
}

func parseDecimal(value string, places int) (int64, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("invalid decimal")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if fraction == "" {
			return 0, fmt.Errorf("invalid decimal")
		}
	}
	if len(fraction) > places {
		return 0, fmt.Errorf("excess decimal precision")
	}
	for _, c := range parts[0] + fraction {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid decimal")
		}
	}
	return strconv.ParseInt(parts[0]+fraction+strings.Repeat("0", places-len(fraction)), 10, 64)
}

func (m Money) String() string { return fmt.Sprintf("%d.%02d", m/100, m%100) }

func (m Money) Times(quantity int) (Money, error) {
	if m < 0 || quantity <= 0 || m > MaxMoney || int64(quantity) > int64(MaxMoney)/max(1, int64(m)) {
		return 0, fmt.Errorf("invalid line amount")
	}
	return m * Money(quantity), nil
}

func Add(a, b Money) (Money, error) {
	if a < 0 || b < 0 || a > MaxMoney-b {
		return 0, fmt.Errorf("amount exceeds supported range")
	}
	return a + b, nil
}
