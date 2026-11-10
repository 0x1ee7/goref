package shoplog

import (
	"log/slog"
	"net/http"
)

// RequestIDHeader carries the request id in and out of the service.
const RequestIDHeader = "X-Request-Id"

// AccessLog returns middleware that assigns a request id (honouring a valid
// inbound RequestIDHeader), echoes it in the response, stores it in the
// request context, calls next, and logs exactly one record afterwards. A nil
// logger means slog.Default() at request time. See README, Part 6.
func AccessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return nil
}
