package shard

import (
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
)

func init() { plugin.Register("shard", setup) }

func setup(c *caddy.Controller) error {
	c.Next() // 'demo'
	if c.NextArg() {
		return plugin.Error("shard", c.ArgErr())
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		return newSharder(next)
	})

	return nil
}
