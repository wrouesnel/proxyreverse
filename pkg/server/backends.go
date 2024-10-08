package server

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/imroc/req/v3"
	"github.com/wrouesnel/proxyreverse/pkg/server/config"
	"go.uber.org/zap"
)

type HTTPBackend struct {
	logger     *zap.Logger
	client     *req.Client // client is the HTTP request client used to forward connections
	proxychain Proxychain  // proxychain is the chain of proxies which connect to the system

	target         string
	port           uint16
	tls            config.TLS
	setHeaders     http.Header    // setHeaders ore the headers to set on the outbound request
	delHeaders     []string       // delHeaders are the headers to delete on the outbound request
	targetSelector TargetSelector // targetSelector implements the actual target backend selection logic
}

func NewHTTPBackend(config config.BackendConfig, proxychain Proxychain) (*HTTPBackend, error) {
	client := req.NewClient().
		SetDial(proxychain.Dialer().DialContext)

	sniName := config.Target.Host
	if config.TLS.ServerNameIndication != nil {
		sniName = *config.TLS.ServerNameIndication
	}

	if config.TLS.Enable {
		client = client.SetTLSClientConfig(&tls.Config{
			InsecureSkipVerify: config.TLS.NoVerify,
			ServerName:         sniName,
			RootCAs:            config.TLS.CACerts.CertPool,
		})
	}

	targetSelector := NewTargetSelector(config.TargetSelect, config.TargetSelectParams)
	if targetSelector == nil {
		return nil, fmt.Errorf("invalid target_select specification")
	}

	r := &HTTPBackend{
		client:     client,
		proxychain: proxychain,

		target:         config.Target.Host,
		port:           config.Target.Port,
		tls:            config.TLS,
		setHeaders:     config.HTTPHeaders.SetHeaders,
		delHeaders:     config.HTTPHeaders.DelHeaders,
		targetSelector: targetSelector,
	}
	r.logger = zap.L().With(zap.String("target", config.Target.String()))

	return r, nil
}

// ServerHTTP implements http.Handler.
func (h HTTPBackend) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	// Receive the request, copy headers and make the outbound request.
	outbound := h.client.NewRequest()
	outbound.Method = request.Method
	outbound.Headers = request.Header
	// Set headers
	for k, v := range h.setHeaders {
		outbound.Headers[k] = v
	}
	// Delete headers we don't want
	for _, k := range h.delHeaders {
		delete(outbound.Headers, k)
	}

	scheme := "http"
	if h.tls.Enable {
		scheme = "https"
	}

	// Get the target name
	target := h.targetSelector.GetTarget(h, request)
	targetHost, _, _ := net.SplitHostPort(target)

	if h.target == "" && h.tls.Enable {
		// Need to set SNI name per request if no specified target
		tlsConfig := h.client.GetTLSClientConfig()
		tlsConfig.ServerName = targetHost
		h.client.SetTLSClientConfig(tlsConfig)
	}

	outboundURL := url.URL{
		Scheme:      scheme,
		User:        request.URL.User,
		Host:        target,
		Path:        request.URL.Path,
		RawQuery:    request.URL.RawQuery,
		RawFragment: request.URL.RawFragment,
	}
	outbound.RawURL = outboundURL.String()
	outbound.GetBody = func() (io.ReadCloser, error) {
		return request.Body, nil
	}

	// Do the outbound request
	resp := outbound.Do(request.Context())

	// Read response headers
	headerMap := writer.Header()
	if resp.Response == nil {
		h.logger.Debug("Error contacting backend", zap.Error(resp.Err))
		writer.WriteHeader(http.StatusBadGateway)
		return
	}

	if resp.Response.Header != nil {
		for k, v := range resp.Response.Header {
			headerMap[k] = v
		}
	}
	// Write response headers
	writer.WriteHeader(resp.StatusCode)
	// Copy response body from host to destination
	io.Copy(writer, resp.Response.Body)
}
