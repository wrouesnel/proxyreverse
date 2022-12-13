package server

import (
	"fmt"
	"net/url"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	connect_proxy_scheme "github.com/wrouesnel/go.connect-proxy-scheme"
	"github.com/wrouesnel/proxyreverse/pkg/server/config"
	"go.uber.org/zap"
	"golang.org/x/net/proxy"
)

var (
	ErrDirectProxyAfterNonDirectProxy = errors.New("direct connection does not make sense as non-first member of proxychain")
)

// ErrInvalidProxySpec is the common type for proxy construction errors.
type ErrInvalidProxySpec struct {
	cause error
}

func (e ErrInvalidProxySpec) Error() string {
	return fmt.Sprintf("invalid proxy url: %v", e.cause)
}

func (e ErrInvalidProxySpec) Cause() error {
	return e.cause
}

//nolint:gochecknoinits
func init() {
	proxy.RegisterDialerType("http", connect_proxy_scheme.ConnectProxy)
}

// proxychain implements a dialer which chains successive proxies together in
// order to reach a target addr.
type proxychain struct {
	dialer proxy.ContextDialer
}

// Dialer implements Proxychain.
func (pc *proxychain) Dialer() proxy.ContextDialer {
	return pc.dialer
}

// Proxychain provides an interface to constructed chains of proxies.
type Proxychain interface {
	Dialer() proxy.ContextDialer
}

// NewProxychainFromConfig creates a new proxychain from the supplied
// list of configs.
func NewProxychainFromConfig(cfg []config.Proxy) (Proxychain, error) {
	logger := zap.L()
	// Initial dialer is a direct dialer
	var proxyDialer proxy.Dialer = proxy.Direct

	// Loop through the chain and wrap each stage
	for idx, proxyConf := range cfg {
		llogger := logger.With(zap.String("proxy_url", (string)(proxyConf.Proxy)))
		llogger.Debug("Construct proxy dialer")
		switch proxyConf.Proxy {
		case config.ProxyDirect:
			if idx == 0 {
				// Skip - we always start out direct
				continue
			} else {
				// Error - this makes no sense.
				llogger.Error("Direct proxy does not make sense when not first element of proxychain")
				return nil, &ErrInvalidProxySpec{ErrDirectProxyAfterNonDirectProxy}
			}
		case config.ProxyEnvironment:
			llogger.Debug("Proxy from environment")
			newDialer := proxy.FromEnvironmentUsing(proxyDialer)
			proxyDialer = newDialer
		default:
			llogger.Debug("Proxy from explicit URL")
			proxyURL := lo.Must(url.Parse((string)(proxyConf.Proxy)))
			newDialer, err := proxy.FromURL(proxyURL, proxyDialer)
			if err != nil {
				llogger.Error("Proxy from URL failed")
				return nil, &ErrInvalidProxySpec{err}
			}
			proxyDialer = newDialer
		}
	}

	chain := proxychain{}
	chain.dialer = proxyDialer.(proxy.ContextDialer)

	return &chain, nil
}
