package timeutil

import "errors"

// Sentinel errors. Wrap them with %w so callers can test with errors.Is.
var (
	// ErrInvalid reports an invalid argument: a nil location, an out-of-range
	// field, a non-positive interval, a negative count, and so on.
	ErrInvalid = errors.New("invalid argument")

	// ErrUnknownZone reports a zone name that cannot be loaded.
	ErrUnknownZone = errors.New("unknown time zone")

	// ErrNonexistent reports a wall-clock time skipped by a DST transition
	// when the policy is GapSkip.
	ErrNonexistent = errors.New("wall-clock time does not exist")

	// ErrAmbiguous reports a wall-clock time repeated by a DST transition when
	// the policy is OverlapSkip.
	ErrAmbiguous = errors.New("wall-clock time is ambiguous")

	// ErrInvalidTimestamp reports input that is not a timestamp with a
	// determinable instant.
	ErrInvalidTimestamp = errors.New("invalid timestamp")

	// ErrUnknownOffset reports a timestamp whose UTC offset is declared
	// unknown ("-0000" in RFC 2822, "-00:00" in RFC 3339).
	ErrUnknownOffset = errors.New("timestamp has unknown UTC offset")

	// ErrOverflow reports a Duration result outside the representable range.
	ErrOverflow = errors.New("duration overflow")

	// ErrAlreadyRunning reports Start on a running Stopwatch.
	ErrAlreadyRunning = errors.New("stopwatch already running")

	// ErrNotRunning reports Stop on a Stopwatch that is not running.
	ErrNotRunning = errors.New("stopwatch not running")
)
