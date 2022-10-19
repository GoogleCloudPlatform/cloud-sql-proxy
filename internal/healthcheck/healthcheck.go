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
	"sync"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cloudsql"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/proxy"
)

// Check provides HTTP handlers for use as healthchecks typically in a
// Kubernetes context.
type Check struct {
	once     *sync.Once
	started  chan struct{}
	proxy    *proxy.Client
	logger   cloudsql.Logger
	minReady uint64
}

// NewCheck is the initializer for Check.
func NewCheck(p *proxy.Client, l cloudsql.Logger, minReady uint64) *Check {
	return &Check{
		once:     &sync.Once{},
		started:  make(chan struct{}),
		proxy:    p,
		logger:   l,
		minReady: minReady,
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
func (c *Check) HandleReadiness(w http.ResponseWriter, _ *http.Request) {
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

	err := c.proxy.CheckConnections(ctx)
	if err != nil && !ready(err, c.minReady) {
		c.logger.Errorf("[Health Check] Readiness failed: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func ready(err error, minReady uint64) bool {
	mErr, ok := err.(proxy.MultiErr)
	if !ok {
		return false
	}
	if uint64(len(mErr)) > minReady {
		return false
	}
	return true
}

// HandleLiveness indicates the process is up and responding to HTTP requests.
// If this check fails (because it's not reachable), the process is in a bad
// state and should be restarted.
func (c *Check) HandleLiveness(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
