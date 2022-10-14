// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package tests contains end to end tests meant to verify the Cloud SQL Auth proxy
// works as expected when executed as a binary.
//
// Required flags:
//
//	-mysql_conn_name, -db_user, -db_pass
package tests

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cmd"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/log"
)

var (
	impersonatedUser = flag.String(
		"impersonated_user",
		os.Getenv("IMPERSONATED_USER"),
		"Name of the service account that supports impersonation (impersonator must have roles/iam.serviceAccountTokenCreator)",
	)
)

// ProxyExec represents an execution of the Cloud SQL proxy.
type ProxyExec struct {
	Out io.ReadCloser

	cmd     *cmd.Command
	cancel  context.CancelFunc
	closers []io.Closer
	done    chan bool // closed once the cmd is completed
	err     error
}

// StartProxy returns a proxyExec representing a running instance of the proxy.
func StartProxy(ctx context.Context, args ...string) (*ProxyExec, error) {
	ctx, cancel := context.WithCancel(ctx)
	// Open a pipe for tracking the output from the cmd
	pr, pw, err := os.Pipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("unable to open stdout pipe: %w", err)
	}

	c := cmd.NewCommand(cmd.WithLogger(log.NewStdLogger(pw, pw)))
	c.SetArgs(args)
	c.SetOut(pw)
	c.SetErr(pw)

	p := &ProxyExec{
		Out:     pr,
		cmd:     c,
		cancel:  cancel,
		closers: []io.Closer{pr, pw},
		done:    make(chan bool),
	}
	// Start the command in the background
	go func() {
		defer close(p.done)
		defer cancel()
		p.err = c.ExecuteContext(ctx)
	}()
	return p, nil
}

// Stop sends the TERM signal to the proxy and returns.
func (p *ProxyExec) Stop() {
	p.cancel()
}

// Waits until the execution is completed and returns any error.
func (p *ProxyExec) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.done:
		return p.err
	}
}

// Done returns true if the proxy has exited.
func (p *ProxyExec) Done() bool {
	select {
	case <-p.done:
		return true
	default:
	}
	return false
}

// Close releases any resources associated with the instance.
func (p *ProxyExec) Close() {
	p.cancel()
	for _, c := range p.closers {
		c.Close()
	}
}

// WaitForServe waits until the proxy ready to serve traffic. Returns any output from
// the proxy while starting or any errors experienced before the proxy was ready to
// server.
func (p *ProxyExec) WaitForServe(ctx context.Context) (output string, err error) {
	// Watch for the "Ready for new connections" to indicate the proxy is listening
	buf, in, errCh := new(bytes.Buffer), bufio.NewReader(p.Out), make(chan error, 1)
	go func() {
		defer close(errCh)
		for {
			// if ctx is finished, stop processing
			select {
			case <-ctx.Done():
				return
			default:
			}
			s, err := in.ReadString('\n')
			if err != nil {
				errCh <- err
				return
			}
			if _, err = buf.WriteString(s); err != nil {
				errCh <- err
				return
			}
			// Check for an unrecognized flag
			if strings.Contains(s, "Error") {
				errCh <- errors.New(s)
				return
			}
			if strings.Contains(s, "ready for new connections") {
				errCh <- nil
				return
			}
		}
	}()
	// Wait for either the background thread of the context to complete
	select {
	case <-ctx.Done():
		return buf.String(), ctx.Err()
	case err := <-errCh:
		if err != nil {
			return buf.String(), err
		}
	}
	return buf.String(), nil
}
