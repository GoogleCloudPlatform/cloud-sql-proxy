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
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"testing"
)

var (
	binPath = ""
)

func TestMain(m *testing.M) {
	flag.Parse()
	// compile the proxy as a binary
	var err error
	binPath, err = compileProxy()
	if err != nil {
		log.Fatalf("failed to compile proxy: %s", err)
	}
	// Run tests and cleanup
	rtn := m.Run()
	os.RemoveAll(binPath)

	os.Exit(rtn)
}

// compileProxy compiles the binary into a temporary directory, and returns the path to the file or any error that occured.
func compileProxy() (string, error) {
	// get path of the cmd pkg
	_, f, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to find cmd pkg")
	}
	projRoot := path.Dir(path.Dir(f)) // cd ../..
	pkgPath := path.Join(projRoot, "cmd", "cloud_sql_proxy")
	// compile the proxy into a tmp directory
	tmp, err := ioutil.TempDir("", "")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %s", err)
	}

	b := path.Join(tmp, "cloud_sql_proxy")

	if runtime.GOOS == "windows" {
		b += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", b, pkgPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run 'go build': %w \n %s", err, out)
	}
	return b, nil
}

// proxyExec represents an execution of the Cloud SQL proxy.
type ProxyExec struct {
	Out io.ReadCloser

	cmd     *exec.Cmd
	cancel  context.CancelFunc
	closers []io.Closer
	done    chan bool // closed once the cmd is completed
	err     error
}

// StartProxy returns a proxyExec representing a running instance of the proxy.
func StartProxy(ctx context.Context, args ...string) (*ProxyExec, error) {
	var err error
	ctx, cancel := context.WithCancel(ctx)
	p := ProxyExec{
		cmd:    exec.CommandContext(ctx, binPath, args...),
		cancel: cancel,
		done:   make(chan bool),
	}
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("unable to open stdout pipe: %w", err)
	}
	defer pw.Close()
	p.Out, p.cmd.Stdout, p.cmd.Stderr = pr, pw, pw
	p.closers = append(p.closers, pr)
	if err := p.cmd.Start(); err != nil {
		defer p.Close()
		return nil, fmt.Errorf("unable to start cmd: %w", err)
	}
	// when process is complete, mark as finished
	go func() {
		defer close(p.done)
		p.err = p.cmd.Wait()
	}()
	return &p, nil
}

// Stop sends the pskill signal to the proxy and returns.
func (p *ProxyExec) Kill() {
	p.cancel()
}

// Waits until the execution is completed and returns any error.
func (p *ProxyExec) Wait() error {
	select {
	case <-p.done:
		return p.err
	}
}

// Stop sends the pskill signal to the proxy and returns.
func (p *ProxyExec) Done() bool {
	select {
	case <-p.done:
		return true
	default:
	}
	return false
}

// Close releases any resources assotiated with the instance.
func (p *ProxyExec) Close() {
	p.cancel()
	for _, c := range p.closers {
		c.Close()
	}
}

// WaitForServe waits until the proxy ready to serve traffic. Returns any output from the proxy
// while starting or any errors experienced before the proxy was ready to server.
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
			buf.WriteString(s)
			if strings.Contains(s, "Ready for new connections") {
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
