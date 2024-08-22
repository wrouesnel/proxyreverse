package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type ListenerType string

const (
	SiteConfigTypeHTTPEdge ListenerType = "http-edge"
)

//type SiteType string
//
//const (
//	SiteTypeExact    SiteType = "exact"
//	SiteTypeWildCard SiteType = "wildcard"
//)

type Config struct {
	Global      GlobalConfig              `mapstructure:"global,omitempty"`
	Proxychains map[string][]Proxy        `mapstructure:"proxychains,omitempty"`
	Listeners   map[string]ListenerConfig `mapstructure:"listeners,omitempty"`
	Sites       []SiteConfig              `mapstructure:"sites,omitempty"`
}

type GlobalConfig struct {
	// Site *SiteConfig `mapstructure:"site,omitempty"` // Site is the default global site config
	//Logging           LoggingConfig `mapstructure:"logging,omitempty"`
	//DefaultProxychain string        `mapstructure:"default_proxychain,omitempty"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level,omitempty"`
	Format string `mapstructure:"format,omitempty"`
}

type ListenerConfig struct {
	ListenAddr   HostSpec     `mapstructure:"listen_addr"` // ListenAddr is the hostname and port number
	ListenerType ListenerType `mapstructure:"listen_type"` // ListenerType is the type of listener to attach
}

type SiteConfig struct {
	Listener []string `mapstructure:"listener"` // Listener is the name of the listener to attach the site too
	Host     string   `mapstructure:"host"`     // Host is the hostname to respond to
	// SiteType   SiteType      `mapstructure:"site_type"`  // SiteType is the type of matching to do. Default is exact.
	Backend    BackendConfig `mapstructure:"backend"`    // Backend is the backend for the server
	Proxychain string        `mapstructure:"proxychain"` // Proxychain is the proxychain to use for connections
	Method     string        `mapstructure:"method"`     // Method is the type of proxy to use. Options are "http-edge"
}

type BackendConfig struct {
	Target      HostSpec                      `mapstructure:"target"`
	TLS         TLS                           `mapstructure:"tls,omitempty"` // TLS configures TLS connectivity to the backend
	HTTPHeaders `mapstructure:"http_headers"` // HTTPHeaders configures modifications to the HTTP headers
}

type TLS struct {
	Enable               bool               `mapstructure:"enable"`              // TLS indicates that the connection should be made with TLS
	NoVerify             bool               `mapstructure:"no_verify,omitempty"` // TLSNoVerify means do not verify certificates
	ServerNameIndication *string            `mapstructure:"sni_name,omitempty"`  // The TLS SNI name to send.
	CACerts              TLSCertificatePool `mapstructure:"ca_certs,omitempty"`  // Path to CAfile to verify the service TLS with
}

// HTTPHeaders configures HTTP header modifications.
type HTTPHeaders struct {
	// SetHeaders are headers to set on outbound requests. A common header to set is
	// Host in order to route the request.
	SetHeaders map[string][]string `mapstructure:"set_headers,omitempty"`
	// DelHeaders are a list of headers which should be explicitly removed. Names here are normalized
	// before removal, so spelling does not need to be exact.
	DelHeaders []string `mapstructure:"del_headers,omitempty"`
}

type Proxy struct {
	Proxy ProxyURL `mapstructure:"proxy"`
}

type HostSpec struct {
	Host    string // Host is the hostname
	Port    uint16 // Port is the port number
	Network string // Network type (default TCP)
}

// UnmarshalText implements the TextMarshaller interface for HostSpec.
func (u *HostSpec) UnmarshalText(text []byte) error {
	splitPort := strings.SplitN(string(text), "/", 2)
	if len(splitPort) > 1 {
		u.Network = splitPort[1]
	} else {
		u.Network = "tcp"
	}

	host, port, err := net.SplitHostPort(splitPort[0])
	if err != nil {
		return errors.Wrapf(err, "splitting hostname (%s) failed", splitPort[0])
	}

	numPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return errors.Wrapf(err, "port number (%s) could not convert to uint16", port)
	}

	u.Host = host
	u.Port = uint16(numPort)
	return nil
}

// MarshalText implements the TextMarshaller interface for HostSpec.
func (u *HostSpec) MarshalText() ([]byte, error) {
	return []byte(u.String()), nil
}

func (u *HostSpec) String() string {
	return fmt.Sprintf("%v:%v/%v", u.Host, u.Port, u.Network)
}

func (u *HostSpec) HostPort() string {
	// Allow a blank host spec.
	if u.Host == "" && u.Port == 0 {
		return ""
	}
	return fmt.Sprintf("%v:%v", u.Host, u.Port)
}

// URL is a custom URL type that allows validation at configuration load time.
type URL struct {
	*url.URL
}

// NewURL initializes a new URL object.
func NewURL(url string) (URL, error) {
	u := URL{nil}
	err := u.UnmarshalText([]byte(url))
	return u, err
}

// UnmarshalText implements the TextMarshaller interface for URLs.
func (u *URL) UnmarshalText(text []byte) error {
	urlp, err := url.Parse(string(text))

	if err != nil {
		return errors.Wrap(err, "URL.UnmarshalText failed")
	}
	u.URL = urlp
	return nil
}

// MarshalText implements the TextMarshaller interface for URLs.
func (u *URL) MarshalText() ([]byte, error) {
	if u.URL != nil {
		return []byte(u.String()), nil
	}
	return []byte(""), nil
}

// MarshalYAML implements the yaml.Marshaller interface for URLs.
func (u *URL) MarshalYAML() ([]byte, error) {
	if u.URL != nil {
		return []byte(u.String()), nil
	}
	return []byte(""), nil
}
