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

// Package healthcheck tests and communicates the health of the Cloud SQL Auth Proxy.
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
	startedOnce *sync.Once
	started     chan struct{}
	stoppedOnce *sync.Once
	stopped     chan struct{}
	proxy       *proxy.Client
	logger      cloudsql.Logger
}

// NewCheck is the initializer for Check.
func NewCheck(p *proxy.Client, l cloudsql.Logger) *Check {
	return &Check{
		startedOnce: &sync.Once{},
		started:     make(chan struct{}),
		stoppedOnce: &sync.Once{},
		stopped:     make(chan struct{}),
		proxy:       p,
		logger:      l,
	}
}

// NotifyStarted notifies the check that the proxy has started up successfully.
func (c *Check) NotifyStarted() {
	c.startedOnce.Do(func() { close(c.started) })
}

// NotifyStopped notifies the check that the proxy has started up successfully.
func (c *Check) NotifyStopped() {
	c.stoppedOnce.Do(func() { close(c.stopped) })
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

var (
	errNotStarted = errors.New("proxy is not started")
	errStopped    = errors.New("proxy has stopped")
)

// HandleReadiness ensures the Check has been notified of successful startup,
// that the proxy has not reached maximum connections, and that the Proxy has
// not started shutting down.
func (c *Check) HandleReadiness(w http.ResponseWriter, req *http.Request) {
	select {
	case <-c.started:
	default:
		c.logger.Errorf("[Health Check] Readiness failed: %v", errNotStarted)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(errNotStarted.Error()))
		return
	}

	select {
	case <-c.stopped:
		c.logger.Errorf("[Health Check] Readiness failed: %v", errStopped)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(errStopped.Error()))
		return
	default:
	}

	if open, maxCount := c.proxy.ConnCount(); maxCount > 0 && open == maxCount {
		err := fmt.Errorf("max connections reached (open = %v, max = %v)", open, maxCount)
		c.logger.Errorf("[Health Check] Readiness failed: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(err.Error()))
		return
	}

	var minReady int
	if minReadyStr, ok := req.URL.Query()["min-ready"]; ok {
		var err error
		if minReady, err = strconv.Atoi(minReadyStr[0]); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("error"))
			return
		}
		if minReady > c.proxy.InstanceCount() {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("min ready > instance count"))
			return
		}
	}

	if minReady > 0 {
		if _, err := c.proxy.CheckConnections(context.Background()); err != nil {
			mErr := err.(proxy.MultiErr)

			if minReady > c.proxy.InstanceCount()-len(mErr) {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("min ready"))
				return
			}
		}
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
