package server

import (
	"context"
	"net"
	"net/http"

	"github.com/MadAppGang/httplog"
	lzap "github.com/MadAppGang/httplog/zap"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Listener interface {
	AddSite(host string, backend http.Handler) error
}

type HTTPEdgeListener struct {
	logger   *zap.Logger
	server   *http.Server
	backends map[string]http.Handler
}

func (l *HTTPEdgeListener) AddSite(host string, backend http.Handler) error {
	l.backends[host] = backend
	return nil
}

// handler implements HandlerFunc.
func (l *HTTPEdgeListener) handler(w http.ResponseWriter, r *http.Request) {
	hostname, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		hostname = r.Host
	}

	backend, found := l.backends[hostname]
	// Bad gateway
	if !found {
		l.logger.Debug("Host is not known", zap.String("hostname", hostname))
		r.Body.Close()
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	// Dispatch to the correct backend
	backend.ServeHTTP(w, r)
}

func NewHTTPEdgeListener(ctx context.Context, cfg listenerKey) (Listener, error) {
	r := &HTTPEdgeListener{
		logger:   zap.L().With(zap.String("addr", cfg.Addr.String()), zap.String("network", cfg.Network)),
		backends: map[string]http.Handler{},
	}

	logger := r.logger

	listener, err := net.Listen(cfg.Network, cfg.Addr.String())
	if err != nil {
		logger.Error("Could not start listener", zap.Error(err))
		return nil, errors.Wrapf(err, "failed to start listener: %v/%v", cfg.Addr.String(), cfg.Network)
	}

	handler := httplog.LoggerWithConfig(httplog.LoggerConfig{
		Formatter: lzap.ZapLogger(r.logger, zap.InfoLevel, "HTTP Request"),
	}, http.HandlerFunc(r.handler))

	r.server = &http.Server{
		Handler: handler,
	}

	go func() {
		err := r.server.Serve(listener)
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Got error starting HTTP server", zap.Error(err))
		} else {
			logger.Info("Server closed")
		}
	}()

	go func() {
		<-ctx.Done()
		logger.Info("HTTP request server shutdown")
		if err := r.server.Shutdown(context.Background()); err != nil {
			logger.Error("Got error while closing HTTP server", zap.Error(err))
		}
		logger.Info("HTTP server shutdown successful")
	}()

	return r, nil
}
