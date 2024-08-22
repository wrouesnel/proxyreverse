[![Build and Test](https://github.com/wrouesnel/proxyreverse/actions/workflows/integration.yml/badge.svg)](https://github.com/wrouesnel/proxyreverse/actions/workflows/integration.yml)
[![Release](https://github.com/wrouesnel/proxyreverse/actions/workflows/release.yml/badge.svg)](https://github.com/wrouesnel/proxyreverse/actions/workflows/release.yml)
[![Container Build](https://github.com/wrouesnel/proxyreverse/actions/workflows/container.yml/badge.svg)](https://github.com/wrouesnel/proxyreverse/actions/workflows/container.yml)
[![Coverage Status](https://coveralls.io/repos/github/wrouesnel/proxyreverse/badge.svg?branch=main)](https://coveralls.io/github/wrouesnel/proxyreverse?branch=main)


# ProxyReverse

ProxyReverse is a very simple reverse proxy implementation that can proxy to servers
which are themselves behind an HTTP forward proxy.

This is a common annoyance with corporate firewalls, and a nuisance to solve with
standard tools.

## Usage

### Basic Reverse Proxy

Basic usage is simple: define your config file and run the reverse-proxy:

```shell
proxyreverse reverse-proxy
```

The included config file shows a simple example of presenting HTTPS Google on
`localhost:8080` - which isn't very useful, but illustrates the idea of what
we're trying to make easier to do.

### Wildcard Reverse Proxy

It is possible to configure the system to act as essentially a forward proxy
by setting a wildcard site configuration. In this mode, one or more hostname
components are replaced by wildcards. The most specific handler for each request
is used - i.e. if the following config is used...

```yaml
sites:
 - host: "*.com"
   listener:
   - http
   proxychain: default
 - host: "*.*.com"
   backend:
     target: "google.com:443"
     http_headers:
       set_headers:
         Host:
           - google.com
     tls:
       enable: true
       sni_name: google.com
       ca_certs:
         - system
   listener:
     - http
   proxychain: default
```

...then the result of this config will be that a request with a Host header 
of `fake.example.com` will be proxied directly to the `google.com` backend.

Whereas a request to `example.com` - in this case since no `target` field is set
- will be proxied directly (via the default chain) to `example.com` without
modification.

This is a synthetic example: the original purpose of the wildcard functionality
is to provide for setting up a quick and easy backend to a custom top-level
domain behind a SOCKS proxy: e.g. routing `*.onion` addresses to Tor or routing
`*.internal` addresses through an underlying SSH tunnel.

Note: Backend port selection matters and several parameters are provided to
control it. By default `proxyreverse` will guess the backend port based on the
port number of the listener it is attached to.

If an explicit `target` is set, then that port number will be used instead.
`target` can be set to an empty host and just a port number by the following
syntax:

```
target: ":80"
```

which will lead to all forwarded requests using port 80 for the outbound.

### Path Proxying

There is limited support for controlling the selection of the backend target
dynamically via the `target_select` directive. By using the `path` selector,
the backend host can be chosen from a path element rather then via the `Host`
header as per normal - this allows (in a very simple way) to front a backend
as a sub-URL path.

Note: if you need advanced functionality including content adaptation, use a
proper reverse proxy from `nginx`.