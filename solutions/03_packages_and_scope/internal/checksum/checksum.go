// Package checksum computes and verifies Luhn check digits for decimal
// identifiers such as account numbers. It lives under internal/, so only code
// inside the exercise tree may import it.
package checksum

import (
	"errors"
	"fmt"
)

var errNotDigits = errors.New("payload must be non-empty ASCII decimal digits")

// Digit returns the Luhn check digit ('0'..'9') that must follow payload.
// payload must be a non-empty string of ASCII decimal digits; otherwise an
// error is returned.
func Digit(payload string) (byte, error) {
	if !isDigits(payload) {
		return 0, fmt.Errorf("%w: %q", errNotDigits, payload)
	}
	// The check digit sits to the right of payload, so the rightmost payload
	// digit is the first one doubled.
	sum := 0
	double := true
	for i := len(payload) - 1; i >= 0; i-- {
		d := int(payload[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return byte('0' + (10-sum%10)%10), nil
}

// Append returns payload followed by its Luhn check digit.
func Append(payload string) (string, error) {
	d, err := Digit(payload)
	if err != nil {
		return "", err
	}
	return payload + string(d), nil
}

// Valid reports whether number is at least two decimal digits long and its
// final digit is the correct check digit for the digits before it.
func Valid(number string) bool {
	n := len(number)
	if n < 2 || !isDigits(number) {
		return false
	}
	d, err := Digit(number[:n-1])
	return err == nil && d == number[n-1]
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
