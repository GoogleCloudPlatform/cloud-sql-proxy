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

// Package util contains utility functions for use throughout the Cloud SQL Proxy.
package util

import "strings"

// SplitName splits a fully qualified instance into its project, region, and
// instance name components. While we make the transition to regionalized
// metadata, the region is optional.
//
// Examples:
//    "proj:region:my-db" -> ("proj", "region", "my-db")
//		"google.com:project:region:instance" -> ("google.com:project", "region", "instance")
//		"google.com:missing:part" -> ("google.com:missing", "", "part")
func SplitName(instance string) (project, region, name string) {
	spl := strings.Split(instance, ":")
	if len(spl) < 2 {
		return "", "", instance
	}
	if dot := strings.Index(spl[0], "."); dot != -1 {
		spl[1] = spl[0] + ":" + spl[1]
		spl = spl[1:]
	}
	switch {
	case len(spl) < 2:
		return "", "", instance
	case len(spl) == 2:
		return spl[0], "", spl[1]
	default:
		return spl[0], spl[1], spl[2]
	}
}
