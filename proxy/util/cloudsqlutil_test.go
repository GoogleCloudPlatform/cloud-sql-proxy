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

package util

import "testing"

func TestSplitName(t *testing.T) {
	table := []struct{ in, wantProj, wantRegion, wantInstance string }{
		{"proj:region:my-db", "proj", "region", "my-db"},
		{"google.com:project:region:instance", "google.com:project", "region", "instance"},
		{"google.com:missing:part", "google.com:missing", "", "part"},
	}

	for _, test := range table {
		gotProj, gotRegion, gotInstance := SplitName(test.in)
		if gotProj != test.wantProj {
			t.Errorf("splitName(%q): got %v for project, want %v", test.in, gotProj, test.wantProj)
		}
		if gotRegion != test.wantRegion {
			t.Errorf("splitName(%q): got %v for region, want %v", test.in, gotRegion, test.wantRegion)
		}
		if gotInstance != test.wantInstance {
			t.Errorf("splitName(%q): got %v for instance, want %v", test.in, gotInstance, test.wantInstance)
		}
	}
}
