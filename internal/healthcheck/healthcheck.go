// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package healthcheck tests and communicates the health of the Cloud SQL Auth proxy.
package healthcheck

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cloudsql"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
)

// Check provides HTTP handlers for use as healthchecks typically in a
// Kubernetes context.
type Check struct {
	once    *sync.Once
	started chan struct{}
	proxy   *proxy.Client
	logger  cloudsql.Logger
}

// NewCheck is the initializer for Check.
func NewCheck(p *proxy.Client, l cloudsql.Logger) *Check {
	return &Check{
		once:    &sync.Once{},
		started: make(chan struct{}),
		proxy:   p,
		logger:  l,
	}
}

// NotifyStarted notifies the check that the proxy has started up successfully.
func (c *Check) NotifyStarted() {
	c.once.Do(func() { close(c.started) })
}

// HandleStartup reports whether the Check has been notified of startup.
func (c *Check) HandleStartup(w http.ResponseWriter, _ *http.Request) {
	select {
	case <-c.started:
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("error"))
	}
}

var errNotStarted = errors.New("proxy is not started")

// HandleReadiness ensures the Check has been notified of successful startup,
// that the proxy has not reached maximum connections, and that all connections
// are healthy.
func (c *Check) HandleReadiness(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	select {
	case <-c.started:
	default:
		c.logger.Errorf("[Health Check] Readiness failed: %v", errNotStarted)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(errNotStarted.Error()))
		return
	}

	if open, max := c.proxy.ConnCount(); max > 0 && open == max {
		err := fmt.Errorf("max connections reached (open = %v, max = %v)", open, max)
		c.logger.Errorf("[Health Check] Readiness failed: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(err.Error()))
		return
	}

	var minReady *int
	q := req.URL.Query()
	if v := q.Get("min-ready"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			c.logger.Errorf("[Health Check] min-ready must be a valid integer, got = %q", v)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "min-query must be a valid integer, got = %q", v)
			return
		}
		if n <= 0 {
			c.logger.Errorf("[Health Check] min-ready %q must be greater than zero", v)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "min-query must be greater than zero", v)
			return
		}
		minReady = &n
	}

	total, err := c.proxy.CheckConnections(ctx)

	switch {
	case minReady != nil && *minReady > total:
		// When min ready is set and exceeds total instances, 400 status.
		mErr := fmt.Errorf(
			"min-ready (%v) must be less than or equal to the number of registered instances (%v)",
			*minReady, total,
		)
		c.logger.Errorf("[Health Check] Readiness failed: %v", mErr)

		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(mErr.Error()))
		return
	case err != nil && minReady != nil:
		// When there's an error and min ready is set, AND min ready instances
		// are not ready, 503 status.
		c.logger.Errorf("[Health Check] Readiness failed: %v", err)

		mErr, ok := err.(proxy.MultiErr)
		if !ok {
			// If the err is not a MultiErr, just return it as is.
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(err.Error()))
			return
		}

		areReady := total - len(mErr)
		if areReady < *minReady {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(err.Error()))
			return
		}
	case err != nil:
		// When there's just an error without min-ready: 503 status.
		c.logger.Errorf("[Health Check] Readiness failed: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(err.Error()))
		return
	}

	// No error cases apply, 200 status.
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// HandleLiveness indicates the process is up and responding to HTTP requests.
// If this check fails (because it's not reachable), the process is in a bad
// state and should be restarted.
func (c *Check) HandleLiveness(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
