package server

import (
	"context"
	"github.com/wrouesnel/proxyreverse/assets"
	"github.com/wrouesnel/proxyreverse/pkg/server/config"
)

type ServerCommand struct {
}

// Server implements the Pathfinding Proxy Server.
func Server(ctx context.Context, assets assets.Config, sc ServerCommand, config config.Config) error {

	return nil
}
