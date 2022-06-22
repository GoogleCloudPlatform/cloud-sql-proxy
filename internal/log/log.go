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

package log

import (
	"io"
	llog "log"
)

// Logger is the interface used throughout the project for logging.
type Logger interface {
	// Infof is for reporting informational messages.
	Infof(format string, args ...interface{})
	// Errorf is for reporitng errors.
	Errorf(format string, args ...interface{})
}

// StdLogger logs informational messages and errors.
type StdLogger struct {
	infoLog *llog.Logger
	errLog  *llog.Logger
}

// NewStdLogger create a Logger that uses out and err for informational and
// error messages.
func NewStdLogger(out, err io.Writer) Logger {
	return &StdLogger{
		infoLog: llog.New(out, "", llog.LstdFlags),
		errLog:  llog.New(err, "", llog.LstdFlags),
	}
}

func (l *StdLogger) Infof(format string, v ...interface{}) {
	l.infoLog.Printf(format, v...)
}

func (l *StdLogger) Errorf(format string, v ...interface{}) {
	l.errLog.Printf(format, v...)
}
