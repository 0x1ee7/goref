package deployment

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

// Run serves srv on ln until ctx is cancelled, then shuts down gracefully: the
// listener closes at once and in-flight requests get shutdownTimeout to finish.
// If they overrun it, connections are closed and the error wraps
// context.DeadlineExceeded. Run returns only after Serve has returned.
// A nil error means a clean shutdown.
func Run(ctx context.Context, srv *http.Server, ln net.Listener, shutdownTimeout time.Duration) error {
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		// Serve only returns ErrServerClosed after Shutdown or Close, which we have not called yet.
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	// ctx is already done, so derive the shutdown deadline from a context without its cancellation.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close() // force-close connections that outlived the deadline
		<-serveErr
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
