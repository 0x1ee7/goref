// Package timeutil is the time exercise: a scheduling and billing backend that
// receives timestamps from logs, clients, and payment providers in mixed
// formats, recurs meetings in local time across DST, merges bookings across
// zones, and bills on month ends.
//
// Rules the package follows:
//   - Every "now" is injected (a Clock or an explicit argument). Nothing reads
//     the machine's local zone.
//   - Instants are compared with Equal, Before, or After, and merged in UTC.
//   - Calendar dates are time.Time values at UTC midnight. The time of day and
//     zone of a date argument are ignored.
//   - Invalid arguments return errors wrapping ErrInvalid. Nothing panics.
package timeutil
