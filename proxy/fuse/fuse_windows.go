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

// Package fuse is a package stub for windows, which does not support FUSE.
package fuse

import (
	"errors"
	"io"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/proxy/proxy"
)

func Supported() bool {
	return false
}

func NewConnSrc(mountdir, tmpdir string, client *proxy.Client, connset *proxy.ConnSet) (<-chan proxy.Conn, io.Closer, error) {
	return nil, nil, errors.New("fuse not supported on windows")
}
