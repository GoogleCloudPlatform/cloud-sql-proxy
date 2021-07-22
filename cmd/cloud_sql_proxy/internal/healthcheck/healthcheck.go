// Copyright 2021 Google LLC All Rights Reserved.
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
	"net"
	"net/http"
	"sync"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/logging"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/proxy"
)

const (
	startupPath   = "/startup"
	livenessPath  = "/liveness"
	readinessPath = "/readiness"
)

// Server is a type used to implement health checks for the proxy.
type Server struct {
	// started is used to indicate whether the proxy has finished starting up.
	// If started is open, startup has not finished. If started is closed,
	// startup is complete.
	started chan struct{}
	// once ensures that started can only be closed once.
	once *sync.Once
	// port designates the port number on which Server listens and serves.
	port string
	// srv is a pointer to the HTTP server used to communicated proxy health.
	srv *http.Server
}

// NewServer initializes a Server object and exposes HTTP endpoints used to
// communicate proxy health.
func NewServer(c *proxy.Client, port string) (*Server, error) {
	mux := http.NewServeMux()

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	hcServer := &Server{
		started: make(chan struct{}),
		once:    &sync.Once{},
		port:    port,
		srv:     srv,
	}

	mux.HandleFunc(startupPath, func(w http.ResponseWriter, _ *http.Request) {
		if !hcServer.proxyStarted() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("error"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc(readinessPath, func(w http.ResponseWriter, _ *http.Request) {
		if !isReady(c, hcServer) {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("error"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mux.HandleFunc(livenessPath, func(w http.ResponseWriter, _ *http.Request) {
		if !isLive() { // Because isLive() always returns true, this case should not be reached.
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("error"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.Errorf("Failed to start health check HTTP server: %v", err)
		}
	}()

	return hcServer, nil
}

// Close gracefully shuts down the HTTP server belonging to the Server object.
func (s *Server) Close(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

// NotifyStarted tells the Server that the proxy has finished startup.
func (s *Server) NotifyStarted() {
	s.once.Do(func() { close(s.started) })
}

// proxyStarted returns true if started is closed, false otherwise.
func (s *Server) proxyStarted() bool {
	select {
	case <-s.started:
		return true
	default:
		return false
	}
}

// isLive returns true as long as the proxy is running.
func isLive() bool {
	return true
}

// isReady will check the following criteria:
// 1. Finished starting up / been sent the 'Ready for Connections' log.
// 2. Not yet hit the MaxConnections limit, if set.
// 3. Able to dial all instances without error.
func isReady(c *proxy.Client, s *Server) bool {
	// Not ready until we reach the 'Ready for Connections' log.
	if !s.proxyStarted() {
		logging.Errorf("Readiness failed because proxy has not finished starting up.")
		return false
	}

	// Not ready if the proxy is at the optional MaxConnections limit.
	if !c.AvailableConn() {
		logging.Errorf("Readiness failed because proxy has reached the maximum connections limit (%d).", c.MaxConnections)
		return false
	}

	// Not ready if one or more instances cannot be dialed.
	for _, inst := range c.GetInstances() {
		conn, err := c.Dial(inst)
		if err != nil {
			logging.Errorf("Readiness failed because proxy couldn't connect to %q: %v", inst, err)
			return false
		}
		conn.Close()
	}

	return true
}
