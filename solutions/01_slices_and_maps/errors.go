// Package slicesmaps holds data-structure helpers for a small API gateway:
// slice views, money tiers, caches, rate limiting, top-K ranking, layered
// configuration and inventory reconciliation.
package slicesmaps

import "errors"

// Sentinel errors. Wrap them with context; callers test with errors.Is.
var (
	// ErrInvalidConfig reports a constructor argument that cannot describe a working component.
	ErrInvalidConfig = errors.New("invalid configuration")
	// ErrInvalidValue reports a negative, NaN or out-of-range input value.
	ErrInvalidValue = errors.New("invalid value")
	// ErrNoTiers reports a price schedule with no tiers.
	ErrNoTiers = errors.New("no price tiers")
	// ErrInvalidTier reports a malformed price tier.
	ErrInvalidTier = errors.New("invalid price tier")
	// ErrInvalidEnv reports an environment variable that cannot be placed in the config tree.
	ErrInvalidEnv = errors.New("invalid environment variable")
)
