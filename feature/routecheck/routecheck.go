// Copyright (c) Tailscale Inc & contributors
// SPDX-License-Identifier: BSD-3-Clause

// Package routecheck registers support for RouteCheck,
// which checks the reachability of overlapping routers.
package routecheck

import (
	"tailscale.com/ipn/ipnext"
	"tailscale.com/ipn/ipnlocal"
	"tailscale.com/ipn/routecheck"
	"tailscale.com/types/logger"
)

// FeatureName is the name of the feature implemented by this package.
// It is also the [extension] name and the log prefix.
const featureName = "routecheck"

func init() {
	ipnext.RegisterExtension(featureName, func(logf logger.Logf, b ipnext.SafeBackend) (ipnext.Extension, error) {
		return &Extension{
			logf:    logger.WithPrefix(logf, featureName+": "),
			backend: b,
		}, nil
	})
}

func GetExtension(b *ipnlocal.LocalBackend) (_ *Extension, ok bool) {
	return ipnlocal.GetExt[*Extension](b)
}

// Extension implements the [ipnext.Extension] interface.
type Extension struct {
	Client *routecheck.Client

	logf    logger.Logf
	backend ipnext.SafeBackend
	host    ipnext.Host
}

var _ ipnext.Extension = new(Extension)

// Name implements the [ipnext.Extension.Name] interface.
func (e *Extension) Name() string {
	return featureName
}

// Init implements the [ipnext.Extension.Init] interface.
func (e *Extension) Init(h ipnext.Host) error {
	e.host = h

	pinger := e.backend.Sys().Engine.Get()

	c, err := routecheck.NewClient(e.logf, pinger, e.backend.(*ipnlocal.LocalBackend))
	if err != nil {
		return err
	}
	e.Client = c

	return nil
}

// Shutdown implements the [ipnext.Extension.Shutdown] interface.
func (e *Extension) Shutdown() error {
	return nil
}
