package server

import (
	"context"
	"github.com/samber/lo"
	"net"
	"net/http"
	"strings"

	"github.com/MadAppGang/httplog"
	lzap "github.com/MadAppGang/httplog/zap"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const wildcardMatch = "*"

type Listener interface {
	AddSite(host string, backend http.Handler) error
}

// matcher encodes the trie structure used for resolving wildcard domains
type matcher struct {
	backend  http.Handler
	subtrees map[string]*matcher
}

type HTTPEdgeListener struct {
	logger   *zap.Logger
	server   *http.Server
	backends *matcher
}

func (l *HTTPEdgeListener) AddSite(host string, backend http.Handler) error {
	hostComponents := lo.Reverse(strings.Split(host, "."))
	currentMatcher := l.backends
	for _, domain := range hostComponents {
		if nextMatcher, found := currentMatcher.subtrees[domain]; found {
			currentMatcher = nextMatcher
		} else {
			currentMatcher.subtrees[domain] = &matcher{backend: nil, subtrees: make(map[string]*matcher)}
			currentMatcher = currentMatcher.subtrees[domain]
		}
	}

	// This should never happen since the check is made before AddSite is called. So just warn here - we might change
	// semantics someday, but it's not fatal just unexpected.
	if currentMatcher.backend != nil {
		l.logger.Warn("Site backend already exists but is being overridden", zap.String("host", host))
	}

	currentMatcher.backend = backend

	return nil
}

// matchSite tries to find a target host in the backends.
func (l *HTTPEdgeListener) matchSite(host string) http.Handler {
	hostComponents := lo.Reverse(strings.Split(host, "."))
	currentMatcher := l.backends
	wasWildCard := false
	for _, domain := range hostComponents {
		if nextMatcher, found := currentMatcher.subtrees[domain]; found {
			currentMatcher = nextMatcher
			wasWildCard = false
			continue
		}
		// No match - but is there a wildcard?
		if nextMatcher, found := currentMatcher.subtrees[wildcardMatch]; found {
			currentMatcher = nextMatcher
			wasWildCard = true
			continue
		}
		// No match, but was the last match a wildcard?
		if wasWildCard {
			continue
		}
		// No match at all. Stop.
		break
	}

	return currentMatcher.backend
}

// handler implements HandlerFunc.
func (l *HTTPEdgeListener) handler(w http.ResponseWriter, r *http.Request) {
	hostname, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		hostname = r.Host
	}

	// Try a direct lookup
	backend := l.matchSite(hostname)
	if backend == nil {
		// Bad gateway
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
		backends: &matcher{backend: nil, subtrees: make(map[string]*matcher)},
	}

	logger := r.logger

	listener, err := net.Listen(cfg.Network, cfg.Addr.String())
	if err != nil {
		logger.Error("Could not start listener", zap.Error(err))
		return nil, errors.Wrapf(err, "failed to start listener: %v/%v", cfg.Addr.String(), cfg.Network)
	}

	handler := httplog.LoggerWithConfig(httplog.LoggerConfig{
		Formatter: lzap.ZapLogger(r.logger, zap.InfoLevel, "HTTP Request"),
	})(http.HandlerFunc(r.handler))

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
