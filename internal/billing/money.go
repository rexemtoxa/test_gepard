package billing

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"
)

var plainDecimalPattern = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

type Money struct {
	value decimal.Decimal
}

func ParsePositiveMoney(input string) (Money, error) {
	value, err := ParseMoney(input)
	if err != nil {
		return Money{}, err
	}
	if !value.IsPositive() {
		return Money{}, fmt.Errorf("amount must be greater than zero")
	}
	return value, nil
}

func ParseMoney(input string) (Money, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return Money{}, fmt.Errorf("amount is required")
	}
	if strings.HasPrefix(value, "+") || strings.ContainsAny(value, "eE") || !plainDecimalPattern.MatchString(value) {
		return Money{}, fmt.Errorf("amount must be a decimal string")
	}

	parsed, err := decimal.NewFromString(value)
	if err != nil {
		return Money{}, fmt.Errorf("amount must be a decimal string")
	}

	return Money{value: parsed}, nil
}

func (m Money) String() string {
	return m.value.String()
}

func (m Money) Add(other Money) Money {
	return Money{value: m.value.Add(other.value)}
}

func (m Money) Sub(other Money) Money {
	return Money{value: m.value.Sub(other.value)}
}

func (m Money) Neg() Money {
	return Money{value: m.value.Neg()}
}

func (m Money) IsNegative() bool {
	return m.value.IsNegative()
}

func (m Money) IsPositive() bool {
	return m.value.GreaterThan(decimal.Zero)
}

func (m Money) Equal(other Money) bool {
	return m.value.Equal(other.value)
}
