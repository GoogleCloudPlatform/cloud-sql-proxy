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
)

var (
	live  bool
	ready bool
)

func InitHealthCheck() {
	http.HandleFunc("/readiness", func(w http.ResponseWriter, r *http.Request) {
		if readinessTest() {
			w.WriteHeader(200)
			w.Write([]byte("ok\n"))
		} else {
			w.WriteHeader(500)
		}
	})

	http.HandleFunc("/liveness", func(w http.ResponseWriter, r *http.Request) {
		live = livenessTest()
		if live {
			w.WriteHeader(200)
			w.Write([]byte("ok\n"))
		} else {
			w.WriteHeader(500)
		}
	})

	go http.ListenAndServe(":8080", nil)
}

func NotifyReady() {
	ready = true
}

// livenessTest returns true as long as the proxy is running
func livenessTest() bool {
	return true
}

// TODO(monazhn): Proxy is not ready when MaxConnections has been reached
func readinessTest() bool {
	return ready
}