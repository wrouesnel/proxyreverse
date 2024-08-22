package server

import (
	"fmt"
	"net"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/wrouesnel/proxyreverse/pkg/server/config"
	"go.uber.org/zap"
)

// TargetSelector implements determining the target backend for an HTTP edge proxy.
type TargetSelector interface {
	// GetTarget returns the target
	GetTarget(backend HTTPBackend, request *http.Request) string
}

// DefaultSelector logic implements the default (not specificed) selector. Namely
// if the backend does not include a specific Host to target, then the Host on the
// incoming request is used.
type DefaultSelector struct {
}

// GetTarget implements TargetSelector.
func (d DefaultSelector) GetTarget(backend HTTPBackend, request *http.Request) string {
	targetHost := request.Host
	if backend.target != "" {
		targetHost = backend.target
	}

	target := fmt.Sprintf("%s:%v", targetHost, backend.port)
	return target
}

// PathIndexSelector splits the URL path into components and extracts the hostname
// from the given Index. By default, the extracted parameter is removed.
type PathIndexSelector struct {
	Index int `mapstructure:"Index"` // Index is the position of the path parameter
}

// GetTarget implements TargetSelector.
func (p PathIndexSelector) GetTarget(backend HTTPBackend, request *http.Request) string {
	pathParts := strings.Split(request.URL.Path, "/")
	if len(pathParts) < p.Index+1 {
		// Return an empty host - none was specified
		return ""
	}

	targetHost := pathParts[p.Index]
	_, targetPortStr, _ := net.SplitHostPort(targetHost)
	targetPortLong, _ := strconv.ParseUint(targetPortStr, 10, 16)
	targetPort := uint16(targetPortLong)

	if backend.port != 0 {
		targetPort = backend.port
	}

	target := fmt.Sprintf("%s:%v", targetHost, targetPort)

	// Edit the URL in place
	modifiedPathParts := slices.Delete(pathParts, p.Index, p.Index+1)
	request.URL.Path = path.Join(modifiedPathParts...)

	return target
}

func NewTargetSelector(name config.TargetSelectType, parameters map[string]interface{}) TargetSelector {
	logger := zap.L().With(zap.String("target_select", string(name)))

	switch name {
	case config.TargetSelectTypeDefault:
		selector := new(DefaultSelector)
		decoder, err := config.Decoder(selector, false)
		if err != nil {
			logger.Error("Error building decoder", zap.Error(err))
			return nil
		}
		if err := decoder.Decode(parameters); err != nil {
			logger.Error("Error while decoding parameters for selector", zap.Error(err))
			return nil
		}
		return selector
	case config.TargetSelectTypePathIndex:
		selector := new(PathIndexSelector)
		decoder, err := config.Decoder(selector, false)
		if err != nil {
			logger.Error("Error building decoder", zap.Error(err))
			return nil
		}
		if err := decoder.Decode(parameters); err != nil {
			logger.Error("Error while decoding parameters for selector", zap.Error(err))
			return nil
		}
		return selector
	default:
		logger.Error("Unknown target selector requested")
		return nil
	}
}
