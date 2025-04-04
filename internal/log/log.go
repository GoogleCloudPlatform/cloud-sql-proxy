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
	"fmt"
	"io"
	llog "log"
	"log/slog"
	"os"
	"time"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cloudsql"
)

const (
	googLvlKey    = "severity"
	googMsgKey    = "message"
	googSourceKey = "sourceLocation"
	googTimeKey   = "timestamp"
)

// StdLogger is the standard logger that distinguishes between info and error
// logs.
type StdLogger struct {
	stdLog *llog.Logger
	errLog *llog.Logger
}

// NewStdLogger create a Logger that uses out and err for informational and
// error messages.
func NewStdLogger(out, err io.Writer) cloudsql.Logger {
	return &StdLogger{
		stdLog: llog.New(out, "", llog.LstdFlags),
		errLog: llog.New(err, "", llog.LstdFlags),
	}
}

// Infof logs informational messages
func (l *StdLogger) Infof(format string, v ...interface{}) {
	l.stdLog.Printf(format, v...)
}

// Errorf logs error messages
func (l *StdLogger) Errorf(format string, v ...interface{}) {
	l.errLog.Printf(format, v...)
}

// Debugf logs debug messages
func (l *StdLogger) Debugf(format string, v ...interface{}) {
	l.stdLog.Printf(format, v...)
}

// StructuredLogger writes log messages in JSON.
type StructuredLogger struct {
	stdLog *slog.Logger
	errLog *slog.Logger
}

// Infof logs informational messages
func (l *StructuredLogger) Infof(format string, v ...interface{}) {
	l.stdLog.Info(fmt.Sprintf(format, v...))
}

// Errorf logs error messages
func (l *StructuredLogger) Errorf(format string, v ...interface{}) {
	l.errLog.Error(fmt.Sprintf(format, v...))
}

// Debugf logs debug messages
func (l *StructuredLogger) Debugf(format string, v ...interface{}) {
	l.stdLog.Debug(fmt.Sprintf(format, v...))
}

// NewStructuredLogger creates a Logger that logs messages using JSON.
func NewStructuredLogger(quiet bool) cloudsql.Logger {
	var infoHandler, errorHandler slog.Handler
	if quiet {
		infoHandler = slog.DiscardHandler
	} else {
		infoHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:       slog.LevelDebug,
			ReplaceAttr: replaceAttr,
		})
	}
	errorHandler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level:       slog.LevelError,
		ReplaceAttr: replaceAttr,
	})

	l := &StructuredLogger{
		stdLog: slog.New(infoHandler),
		errLog: slog.New(errorHandler),
	}
	return l
}

// replaceAttr remaps default Go logging keys to adhere to LogEntry format
// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry
func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	if groups == nil {
		if a.Key == slog.LevelKey {
			a.Key = googLvlKey
			return a
		} else if a.Key == slog.MessageKey {
			a.Key = googMsgKey
			return a
		} else if a.Key == slog.SourceKey {
			a.Key = googSourceKey
			return a
		} else if a.Key == slog.TimeKey {
			a.Key = googTimeKey
			if a.Value.Kind() == slog.KindTime {
				a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339))
			}
			return a
		}
	}
	return a
}
