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

```shell
proxyreverse reverse-proxy
```