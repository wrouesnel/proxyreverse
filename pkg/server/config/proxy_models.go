package config

import (
	"net/url"

	"github.com/pkg/errors"
)

const (
	ProxyEnvironment ProxyURL = "environment"
	ProxyDirect      ProxyURL = "direct"
)

// ProxyURL is a custom type to validate roxy specifications.
type ProxyURL string

// UnmarshalText implements encoding.UnmarshalText.
func (p *ProxyURL) UnmarshalText(text []byte) error {
	s := string(text)
	if _, err := url.Parse(s); err == nil {
		*p = ProxyURL(s)
		return errors.Wrapf(err, "ProxyURL UnmarshalText")
	}
	switch s {
	case (string)(ProxyDirect), (string)(ProxyEnvironment):
		*p = ProxyURL(s)
		return nil
	default:
		return nil
	}
}

// UnmarshalText MarshalText encoding.UnmarshalText.
func (p *ProxyURL) MarshalText() ([]byte, error) {
	return []byte(*p), nil
}
