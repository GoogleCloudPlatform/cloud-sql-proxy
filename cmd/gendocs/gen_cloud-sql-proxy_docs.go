// Copyright 2024 Google LLC
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

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cmd"
	"github.com/spf13/cobra/doc"
)

func main() {
	if len(os.Args) > 2 {
		fmt.Fprintf(os.Stderr, "usage: %s [output directory]\n", os.Args[0])
		os.Exit(1)
	}

	path := "docs/cmd"
	if len(os.Args) == 2 {
		path = os.Args[1]
	}

	outDir, err := filepath.Abs(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get output directory: %v\n", err)
		os.Exit(1)
	}

	// Set environment variables used so the output is consistent,
	// regardless of where we run.
	os.Setenv("TMPDIR", "/tmp")

	cloudSQLProxy := cmd.NewCommand()
	cloudSQLProxy.Execute()
	cloudSQLProxy.DisableAutoGenTag = true
	doc.GenMarkdownTree(cloudSQLProxy.Command, outDir)

	// Edit the Markdown file to add release-please tags around the lines that contain
	// the version number:
	//
	// <!-- {x-release-please-start-version} -->
	// https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.20.0/third_party/licenses.tar.gz
	// <!-- {x-release-please-end} -->

	f := filepath.Join(outDir, "cloud-sql-proxy.md")
	b, err := os.ReadFile(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read file: %v\n", err)
		os.Exit(1)
	}

	var out bytes.Buffer
	sc := bufio.NewScanner(bytes.NewReader(b))
	// Example: https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v2.20.0/third_party/licenses.tar.gz
	re := regexp.MustCompile(`https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v\d+\.\d+\.\d+/`)
	for sc.Scan() {
		line := sc.Bytes()
		if re.Match(line) {
			out.WriteString("<!-- {x-release-please-start-version} -->\n")
			out.Write(line)
			out.WriteString("\n")
			out.WriteString("<!-- {x-release-please-end} -->\n")
		} else {
			out.Write(line)
			out.WriteString("\n")
		}
	}

	if err := os.WriteFile(f, out.Bytes(), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write file: %v\n", err)
		os.Exit(1)
	}
}
