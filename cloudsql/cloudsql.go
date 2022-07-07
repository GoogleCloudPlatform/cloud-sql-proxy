// Copyright 2022 Google LLC
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

package cloudsql

import (
	"context"
	"io"
	"net"

	"cloud.google.com/go/cloudsqlconn"
)

// Dialer dials a Cloud SQL instance and returns its database engine version.
type Dialer interface {
	// Dial returns a connection to the specified instance.
	Dial(ctx context.Context, inst string, opts ...cloudsqlconn.DialOption) (net.Conn, error)
	// EngineVersion retrieves the provided instance's database version (e.g.,
	// POSTGRES_14)
	EngineVersion(ctx context.Context, inst string) (string, error)

	io.Closer
}

// Logger is the interface used throughout the project for logging.
type Logger interface {
	// Debugf is for reporting additional information about internal operations.
	Debugf(format string, args ...interface{})
	// Infof is for reporting informational messages.
	Infof(format string, args ...interface{})
	// Errorf is for reporting errors.
	Errorf(format string, args ...interface{})
}
