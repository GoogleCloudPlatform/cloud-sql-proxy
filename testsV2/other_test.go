// Copyright 2022 Google LLC

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     https://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// other_test runs various tests that are database agnostic.
package tests

import (
	"bufio"
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestVersion(t *testing.T) {
	ctx := context.Background()

	data, err := os.ReadFile("../cmd/version.txt")
	if err != nil {
		t.Fatalf("failed to read version.txt: %v", err)
	}
	want := strings.TrimSpace(string(data))

	// Start the proxy
	p, err := StartProxy(ctx, "--version")
	if err != nil {
		t.Fatalf("proxy start failed: %v", err)
	}
	defer p.Close()

	// Assume the proxy should be able to print "version" relatively quickly
	ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	err = p.Wait(ctx)
	if err != nil {
		t.Fatalf("proxy exited unexpectedly: %v", err)
	}
	output, err := bufio.NewReader(p.Out).ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read output from proxy: %v", err)
	}
	if !strings.Contains(output, want) {
		t.Errorf("proxy did not return correct version: want %q, got %q", want, output)
	}
}
