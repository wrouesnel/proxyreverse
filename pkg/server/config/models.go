package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"

	"github.com/pkg/errors"
)

type Config struct {
	Global      GlobalConfig     `mapstructure:"global"`
	Proxychains map[string][]URL `mapstructure:"proxychains"`
}

type GlobalConfig struct {
	Logging           LoggingConfig `mapstructure:"logging"`
	DefaultProxychain string        `mapstructure:"default_proxychain"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type SiteConfig struct {
	Host        HostSpec `mapstructure:"host"`         // Host is the hostname and port number
	TLSNoVerify bool     `mapstructure:"tls_noverify"` // TLSNoVerify means do not verify certificates
	Proxychain  string   `mapstructure:"proxies"`      // Proxychain is the proxychain to use for connections
}

type HostSpec struct {
	Host string // Host is the hostname
	Port uint16 // Port is the port number
}

// UnmarshalText implements the TextMarshaller interface for HostSpec
func (u *HostSpec) UnmarshalText(text []byte) error {
	host, port, err := net.SplitHostPort(string(text))
	if err != nil {
		return errors.Wrapf(err, "splitting hostname (%s) failed", string(text))
	}

	numPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return errors.Wrapf(err, "port number (%s) could not convert to uint16", port)
	}

	u = new(HostSpec)
	u.Host = host
	u.Port = uint16(numPort)
	return nil
}

// MarshalText implements the TextMarshaller interface for HostSpec
func (u *HostSpec) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%v:%v", u.Host, u.Port)), nil
}

// URL is a custom URL type that allows validation at configuration load time.
type URL struct {
	*url.URL
}

func NewURL(url string) (URL, error) {
	u := URL{nil}
	err := u.UnmarshalText([]byte(url))
	return u, err
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for URLs.
func (u *URL) UnmarshalText(text []byte) error {
	urlp, err := url.Parse(string(text))

	if err != nil {
		return errors.Wrap(err, "URL.UnmarshalText failed")
	}
	u.URL = urlp
	return nil
}

// MarshalYAML implements the yaml.Marshaler interface for URLs.
func (u *URL) MarshalText() ([]byte, error) {
	if u.URL != nil {
		return []byte(u.String()), nil
	}
	return []byte(""), nil
}
