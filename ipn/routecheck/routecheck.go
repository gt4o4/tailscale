// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

// Package routecheck performs status checks for routes from the current host.
package routecheck

import (
	"context"
	"errors"
	"net/netip"
	"time"

	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
	"tailscale.com/types/logger"
	"tailscale.com/types/netmap"
	"tailscale.com/util/mak"
)

// Client generates Reports describing the result of both passive and active
// reachability probing.
type Client struct {
	// Verbose enables verbose logging.
	Verbose bool

	// Logf optionally specifies where to log to.
	// If nil, log.Printf is used.
	Logf logger.Logf

	// These elements are read-only after initialization.
	Pinger Pinger
	b      LocalBackend
}

// Pinger is the interface that wraps the [ipnlocal.LocalBackend.Ping] method.
type Pinger interface {
	Ping(ip netip.Addr, pingType tailcfg.PingType, size int, cb func(*ipnstate.PingResult))
}

// LocalBackend is implemented by [ipnlocal.LocalBackend].
type LocalBackend interface {
	NetMap() *netmap.NetworkMap
	Peers() []tailcfg.NodeView
	WatchNotifications(ctx context.Context, mask ipn.NotifyWatchOpt, onWatchAdded func(), fn func(roNotify *ipn.Notify) (keepGoing bool))
}

// NewClient returns a client that probes its peers using this LocalBackend.
func NewClient(logf logger.Logf, pinger Pinger, b LocalBackend) (*Client, error) {
	if pinger == nil {
		return nil, errors.New("Pinger must be set")
	}
	if b == nil {
		return nil, errors.New("LocalBackend must be set")
	}
	return &Client{
		Logf:   logf,
		Pinger: pinger,
		b:      b,
	}, nil
}

// Report returns the latest reachability report.
// Returns nil if a report isn’t available, which happens during initialization.
func (c *Client) Report() *Report {
	nm := c.b.NetMap()
	if nm == nil {
		return nil // The report wasn’t available.
	}

	// TODO(sfllaw): Return the latest snapshot produced by background probing.
	ctx, _ := context.WithTimeout(context.TODO(), 5*time.Second)
	r, err := c.ProbeAllHARouters(ctx, 5)
	if err != nil {
		c.logf("reachability report error: %v", err)
	}
	return r
}

// RoutersByPrefix represents a map of nodes grouped by the subnet that they route.
type RoutersByPrefix map[netip.Prefix][]tailcfg.NodeView

// RoutersByPrefix returns a map of nodes grouped by the subnet that they route.
// Nodes that route for /0 prefixes are exit nodes, their subnet is the Internet.
// The result omits any prefix that is one of a node’s local addresses.
func (c *Client) RoutersByPrefix() RoutersByPrefix {
	var routers RoutersByPrefix
	for _, n := range c.b.Peers() {
		for _, pfx := range routes(n) {
			mak.Set(&routers, pfx, append(routers[pfx], n))
		}
	}
	return routers
}

// Routes returns a slice of subnets that the given node will route.
// If the node is an exit node, the result will contain at least one /0 prefix.
// If the node is a subnet router, the result will contain a smaller prefix.
// The result omits any prefix that is one of the node’s local addresses.
func routes(n tailcfg.NodeView) []netip.Prefix {
	var routes []netip.Prefix
AllowedIPs:
	for _, pfx := range n.AllowedIPs().All() {
		// Routers never forward their own local addresses.
		for _, addr := range n.Addresses().All() {
			if pfx == addr {
				continue AllowedIPs
			}
		}
		routes = append(routes, pfx)
	}
	return routes
}
