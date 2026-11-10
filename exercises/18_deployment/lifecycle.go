package deployment

import (
	"context"
	"net"
	"net/http"
	"time"
)

// Run serves srv on ln until ctx is cancelled, then shuts down gracefully: the
// listener closes at once and in-flight requests get shutdownTimeout to finish.
// If they overrun it, connections are closed and the error wraps
// context.DeadlineExceeded. Run returns only after Serve has returned.
// A nil error means a clean shutdown; a Serve failure before cancellation is
// returned wrapped.
func Run(ctx context.Context, srv *http.Server, ln net.Listener, shutdownTimeout time.Duration) error {
	return nil
}
