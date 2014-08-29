// Copyright 2014 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

// The metricsmanager package contains implementation for an api facade to
// access metrics functions within state
package metricsmanager

import (
	"github.com/juju/errors"

	"github.com/juju/juju/state/api/base"
)

// Client provides access to the metrics manager api
type Client struct {
	base.ClientFacade
	facade base.FacadeCaller
}

// NewClient creates a new client for accessing the metricsmanager api
func NewClient(st base.APICallCloser) *Client {
	frontend, backend := base.NewClientFacade(st, "MetricsManager")
	return &Client{ClientFacade: frontend, facade: backend}
}

// CleanupOldMetrics looks for metrics that are 24 hours old (or older)
// and have been sent. Any metrics it finds are deleted.
func (c *Client) CleanupOldMetrics() error {
	err := c.facade.FacadeCall("CleanupOldMetrics", nil, nil)
	if err != nil {
		return errors.Trace(err)
	}
	return nil
}
