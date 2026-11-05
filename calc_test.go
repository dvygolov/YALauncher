//go:build windows

package main

import "testing"

func TestEvaluate(t *testing.T) {
	cases := map[string]string{
		"2+2":         "4",
		"2 * 3 + 1":   "7",
		"(1+2)*3":     "9",
		"10/4":        "2.5",
		"2^10":        "1 024",
		"100500/3":    "33 500",
		"sqrt(16)":    "4",
		"-5+2":        "-3",
		"1,5*2":       "3",
		"2 +":         "",
		"chrome":      "",
		"калькулятор": "",
		"1/3":         "0.333333333333",
	}
	for in, want := range cases {
		got, ok := evaluate(in)
		if want == "" {
			if ok {
				t.Errorf("evaluate(%q) = %q, expected failure", in, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("evaluate(%q) = %q (%v), want %q", in, got, ok, want)
		}
	}
}
