package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 30 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// Run обслуживает HTTP до отмены ctx, затем даёт текущим запросам завершиться.
func Run(ctx context.Context, address string, handler http.Handler, logger *slog.Logger) error {
	httpServer := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
	}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- httpServer.ListenAndServe() }()
	logger.Info("listening", "address", address)

	select {
	case err := <-serveErrors:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownContext); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
