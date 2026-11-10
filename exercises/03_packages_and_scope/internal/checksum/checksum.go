// Package checksum computes and verifies Luhn check digits for decimal
// identifiers such as account numbers. It lives under internal/, so only code
// inside the exercise tree may import it.
package checksum

import "errors"

// Digit returns the Luhn check digit ('0'..'9') that must follow payload.
// payload must be a non-empty string of ASCII decimal digits; otherwise an
// error is returned.
func Digit(payload string) (byte, error) {
	return 0, errors.New("not implemented")
}

// Append returns payload followed by its Luhn check digit.
func Append(payload string) (string, error) {
	return "", errors.New("not implemented")
}

// Valid reports whether number is at least two decimal digits long and its
// final digit is the correct check digit for the digits before it.
func Valid(number string) bool {
	return false
}
