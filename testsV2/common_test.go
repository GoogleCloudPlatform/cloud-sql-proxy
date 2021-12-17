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
//    -mysql_conn_name, -db_user, -db_pass
package tests

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/cmd"
	"github.com/spf13/cobra"
)

// proxyExec represents an execution of the Cloud SQL proxy.
type proxyExec struct {
	Out io.ReadCloser

	cmd     *cobra.Command
	cancel  context.CancelFunc
	closers []io.Closer
	done    chan bool // closed once the cmd is completed
	err     error
}

// StartProxy returns a proxyExec representing a running instance of the proxy.
func StartProxy(ctx context.Context, args ...string) (*proxyExec, error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd := cmd.New()
	cmd.SetArgs(args)

	// Open a pipe for tracking the output from the cmd
	pr, pw, err := os.Pipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("unable to open stdout pipe: %w", err)
	}
	// defer pw.Close()
	cmd.SetOut(pw)
	cmd.SetErr(pw)

	p := &proxyExec{
		Out:     pr,
		cmd:     cmd,
		cancel:  cancel,
		closers: []io.Closer{pr, pw},
		done:    make(chan bool),
	}
	// Start the command in the background
	go func() {
		defer close(p.done)
		defer cancel()
		p.err = cmd.ExecuteContext(ctx)
	}()
	return p, nil
}

// Stop sends the TERM signal to the proxy and returns.
func (p *proxyExec) Stop() {
	p.cancel()
}

// Waits until the execution is completed and returns any error.
func (p *proxyExec) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.done:
		return p.err
	}
}

// Stop sends the pskill signal to the proxy and returns.
func (p *proxyExec) Done() bool {
	select {
	case <-p.done:
		return true
	default:
	}
	return false
}

// Close releases any resources associated with the instance.
func (p *proxyExec) Close() {
	p.cancel()
	for _, c := range p.closers {
		c.Close()
	}
}

// WaitForServe waits until the proxy ready to serve traffic. Returns any output from
// the proxy while starting or any errors experienced before the proxy was ready to
// server.
func (p *proxyExec) WaitForServe(ctx context.Context) (output string, err error) {
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
			buf.WriteString(s)
			if strings.Contains(s, "ready for new connections") {
				errCh <- nil
				return
			}
		}
	}()
	// Wait for either the background thread of the context to complete
	select {
	case <-ctx.Done():
		return buf.String(), fmt.Errorf("context done: %w", ctx.Err())
	case err := <-errCh:
		if err != nil {
			return buf.String(), fmt.Errorf("proxy start failed: %w", err)
		}
	}
	return buf.String(), nil
}
