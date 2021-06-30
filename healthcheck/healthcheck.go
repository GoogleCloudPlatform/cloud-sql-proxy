// Copyright 2015 Google Inc. All Rights Reserved.
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
	"net/http"
	"sync/atomic"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
)

type HealthCheck struct {
	live    bool
	ready   bool
	startup bool
}

func InitHealthCheck(proxyClient *proxy.Client) *HealthCheck {

	hc := &HealthCheck{
		live:    true,
		ready:   false,
		startup: false,
	}

	http.HandleFunc("/readiness", func(w http.ResponseWriter, _ *http.Request) {
		hc.ready = readinessTest(proxyClient, hc)
		if hc.ready {
			w.WriteHeader(200)
			w.Write([]byte("ok\n"))
		} else {
			w.WriteHeader(500)
			w.Write([]byte("error\n"))
		}
	})

	http.HandleFunc("/liveness", func(w http.ResponseWriter, _ *http.Request) {
		hc.live = livenessTest()
		if hc.live {
			w.WriteHeader(200)
			w.Write([]byte("ok\n"))
		} else {
			w.WriteHeader(500)
			w.Write([]byte("error\n"))
		}
	})

	go http.ListenAndServe(":8080", nil)

	return hc
}

func NotifyReady(hc *HealthCheck) {
	hc.startup = true
}

// livenessTest returns true as long as the proxy is running
func livenessTest() bool {
	return true
}

// readinessTest checks for the proxy having started up, but not having reached MaxConnections
func readinessTest(proxyClient *proxy.Client, hc *HealthCheck) bool {

	if !hc.startup {
		return false
	}

	// Parts of this code is taken from client.go
	active := atomic.AddUint64(&proxyClient.ConnectionsCounter, 1)

	// Defer decrementing ConnectionsCounter upon connections closing
	defer atomic.AddUint64(&proxyClient.ConnectionsCounter, ^uint64(0))

	if proxyClient.MaxConnections > 0 && active > proxyClient.MaxConnections {
		return false
	}

	return true
}
