package server

import (
	"context"
	"net/netip"

	"github.com/pkg/errors"
	"github.com/wrouesnel/proxyreverse/assets"
	"github.com/wrouesnel/proxyreverse/pkg/server/config"
	"go.uber.org/zap"
)

type ServerCommand struct{}

var (
	ErrNilConfig                  = errors.New("nil config parameter supplied")
	ErrDuplicateListeners         = errors.New("duplicate listener addr and port but different type")
	ErrUnknownListenerType        = errors.New("unknown listener type")
	ErrListenerNotFound           = errors.New("listener not found")
	ErrProxychainNotFound         = errors.New("proxychain for backend is not defined")
	ErrBackendInitFailed          = errors.New("backend initialization is not defined")
	ErrHostListenerClash          = errors.New("duplicate host already attached to listener")
	ErrAttachSiteToListenerFailed = errors.New("failed to attach site to listener")
)

// listenerKey implements the map key used for tracking listeners.
type listenerKey struct {
	Addr    netip.AddrPort
	Network string
}

// siteKey implements the map key used for tracking backends being mapped to listeners.
type siteKey struct {
	Host     string
	Listener string
}

// Server implements the Pathfinding Proxy Server.
func Server(ctx context.Context, assets assets.Config, sc ServerCommand, cfg *config.Config) error {
	logger := zap.L()

	if cfg == nil {
		return ErrNilConfig
	}

	logger.Debug("Constructing proxychains")
	proxychains := map[string]Proxychain{}
	for name, proxychainConfig := range cfg.Proxychains {
		chain, err := NewProxychainFromConfig(proxychainConfig)
		if err != nil {
			return err
		}
		proxychains[name] = chain
	}

	logger.Debug("Constructing the list of addresses to listen on")
	listenPorts := map[listenerKey]string{}
	listenNames := map[listenerKey]string{}
	for listenerName, listenerConfig := range cfg.Listeners {
		listenerLogger := zap.L().With(zap.String("listen_addr", listenerConfig.ListenAddr.String()))

		addr, err := netip.ParseAddr(listenerConfig.ListenAddr.Host)
		if err != nil {
			listenerLogger.Error("Could not parse supplied listen address",
				zap.String("listen_addr", listenerConfig.ListenAddr.Host))
		}

		key := listenerKey{
			Addr:    netip.AddrPortFrom(addr, listenerConfig.ListenAddr.Port),
			Network: listenerConfig.ListenAddr.Network,
		}

		if listenerType, found := listenPorts[key]; found {
			if listenerType != string(listenerConfig.ListenerType) {
				listenerLogger.Error("Duplicate listen addresses with different listener types found",
					zap.String("found", listenerType), zap.String("current", string(listenerConfig.ListenerType)))
				return ErrDuplicateListeners
			}
		}

		if listenerName, found := listenPorts[key]; found {
			listenerLogger.Error("Duplicate listener names found", zap.String("listener_name", listenerName))
			return ErrDuplicateListeners
		}

		listenPorts[key] = string(listenerConfig.ListenerType)
		listenNames[key] = listenerName
	}

	logger.Debug("Starting the listeners")
	listeners := map[string]Listener{}
	for key, listenType := range listenPorts {
		siteLogger := zap.L().With(
			zap.String("addr", key.Addr.String()),
			zap.String("network", key.Network),
			zap.String("listener_type", listenType))

		siteLogger.Debug("Starting listener")

		var listener Listener
		var err error

		switch listenType {
		case (string)(config.SiteConfigTypeHTTPEdge):
			listener, err = NewHTTPEdgeListener(ctx, key)
		default:
			siteLogger.Error("Unimplemented listener type.")
			return errors.Wrapf(ErrUnknownListenerType, "%v", listenType)
		}

		if err != nil {
			siteLogger.Error("Failed to create listener from config")
		}

		listenerName := listenNames[key]
		listeners[listenerName] = listener
	}

	logger.Debug("Initializing backends")
	httpSiteMapping := map[siteKey]*HTTPBackend{}
	for _, siteCfg := range cfg.Sites {
		siteLogger := zap.L().With(zap.String("host", siteCfg.Host))

		pc, found := proxychains[siteCfg.Proxychain]
		if !found {
			siteLogger.Error("Requested proxychain config was not found", zap.String("proxychain", siteCfg.Proxychain))
			return errors.Wrapf(ErrProxychainNotFound, "%v", siteCfg.Proxychain)
		}

		backend, err := NewHTTPBackend(siteCfg.Backend, pc)
		if err != nil {
			siteLogger.Error("Could not initialize backend", zap.Error(err))
			return ErrBackendInitFailed
		}

		for idx, listenerName := range siteCfg.Listener {
			key := siteKey{
				Host:     siteCfg.Host,
				Listener: listenerName,
			}

			if _, found := httpSiteMapping[key]; found {
				siteLogger.Error("Site with matching hostname already attached to this listener",
					zap.String("host", siteCfg.Host), zap.String("listener_name", listenerName),
					zap.Int("site_idx", idx))
				return ErrHostListenerClash
			}

			httpSiteMapping[key] = backend
		}
	}

	logger.Debug("Attaching backends to listeners")
	for key, backend := range httpSiteMapping {
		attachLogger := zap.L().With(
			zap.String("listener_name", key.Listener),
			zap.String("host", key.Host))

		listener, found := listeners[key.Listener]
		if !found {
			attachLogger.Error("Listener was not found when attempting to attach site")
			return ErrListenerNotFound
		}

		if err := listener.AddSite(key.Host, backend); err != nil {
			attachLogger.Error("Failed to attach site to listener", zap.Error(err))
			return ErrAttachSiteToListenerFailed
		}
	}

	logger.Info("Startup complete")
	<-ctx.Done()
	logger.Info("Shutting down")

	return nil
}
