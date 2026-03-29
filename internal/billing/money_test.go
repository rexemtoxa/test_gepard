package billing

import "testing"

func mustParseMoney(t *testing.T, input string) Money {
	t.Helper()

	value, err := ParseMoney(input)
	if err != nil {
		t.Fatalf("ParseMoney(%q) returned error: %v", input, err)
	}

	return value
}

func TestParsePositiveMoney(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "whole amount", input: "10", want: "10"},
		{name: "single fraction digit", input: "10.1", want: "10.1"},
		{name: "two fraction digits", input: "10.15", want: "10.15"},
		{name: "trailing zeroes normalized", input: "10.1500", want: "10.15"},
		{name: "arbitrary scale", input: "0.001", want: "0.001"},
		{name: "zero rejected", input: "0.00", wantErr: true},
		{name: "negative rejected", input: "-1.00", wantErr: true},
		{name: "malformed", input: "abc", wantErr: true},
		{name: "exponent rejected", input: "1e3", wantErr: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParsePositiveMoney(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePositiveMoney(%q) returned error: %v", tc.input, err)
			}
			if got.String() != tc.want {
				t.Fatalf("ParsePositiveMoney(%q) = %q, want %q", tc.input, got.String(), tc.want)
			}
		})
	}
}

func TestParseMoneyAllowsNegativeAndNormalizes(t *testing.T) {
	t.Parallel()

	got, err := ParseMoney(" -0.0010 ")
	if err != nil {
		t.Fatalf("ParseMoney returned error: %v", err)
	}
	if got.String() != "-0.001" {
		t.Fatalf("ParseMoney normalized to %q, want %q", got.String(), "-0.001")
	}
}

func TestMoneyAdd(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		left  string
		right string
		want  string
	}{
		{name: "adds positive amounts", left: "10.25", right: "1.75", want: "12"},
		{name: "adds negative amount", left: "10.25", right: "-1.75", want: "8.5"},
		{name: "preserves zero sum", left: "0", right: "0.00", want: "0"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := mustParseMoney(t, tc.left).Add(mustParseMoney(t, tc.right))
			if got.String() != tc.want {
				t.Fatalf("%s + %s = %s, want %s", tc.left, tc.right, got.String(), tc.want)
			}
		})
	}
}

func TestMoneySub(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		left  string
		right string
		want  string
	}{
		{name: "subtracts smaller amount", left: "10.25", right: "1.75", want: "8.5"},
		{name: "subtracts into negative result", left: "1.75", right: "10.25", want: "-8.5"},
		{name: "subtracts negative amount", left: "10.25", right: "-1.75", want: "12"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := mustParseMoney(t, tc.left).Sub(mustParseMoney(t, tc.right))
			if got.String() != tc.want {
				t.Fatalf("%s - %s = %s, want %s", tc.left, tc.right, got.String(), tc.want)
			}
		})
	}
}

func TestMoneyNeg(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "negates positive amount", input: "10.25", want: "-10.25"},
		{name: "negates negative amount", input: "-10.25", want: "10.25"},
		{name: "negates zero", input: "0.00", want: "0"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := mustParseMoney(t, tc.input).Neg()
			if got.String() != tc.want {
				t.Fatalf("Neg(%s) = %s, want %s", tc.input, got.String(), tc.want)
			}
		})
	}
}

func TestMoneySignMethods(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		input        string
		wantNegative bool
		wantPositive bool
	}{
		{name: "negative amount", input: "-0.01", wantNegative: true, wantPositive: false},
		{name: "zero amount", input: "0.00", wantNegative: false, wantPositive: false},
		{name: "positive amount", input: "0.01", wantNegative: false, wantPositive: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := mustParseMoney(t, tc.input)
			if value.IsNegative() != tc.wantNegative {
				t.Fatalf("IsNegative(%s) = %v, want %v", tc.input, value.IsNegative(), tc.wantNegative)
			}
			if value.IsPositive() != tc.wantPositive {
				t.Fatalf("IsPositive(%s) = %v, want %v", tc.input, value.IsPositive(), tc.wantPositive)
			}
		})
	}
}

func TestMoneyEqual(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		left  string
		right string
		want  bool
	}{
		{name: "equal with different scale", left: "10.0", right: "10.00", want: true},
		{name: "equal negatives", left: "-0.50", right: "-0.5", want: true},
		{name: "different values", left: "10.0", right: "10.01", want: false},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := mustParseMoney(t, tc.left).Equal(mustParseMoney(t, tc.right))
			if got != tc.want {
				t.Fatalf("Equal(%s, %s) = %v, want %v", tc.left, tc.right, got, tc.want)
			}
		})
	}
}
