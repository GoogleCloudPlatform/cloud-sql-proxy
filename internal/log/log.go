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
	"os"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cloudsql"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// StdLogger is the standard logger that distinguishes between info and error
// logs.
type StdLogger struct {
	infoLog *llog.Logger
	errLog  *llog.Logger
}

// NewStdLogger create a Logger that uses out and err for informational and
// error messages.
func NewStdLogger(out, err io.Writer) cloudsql.Logger {
	return &StdLogger{
		infoLog: llog.New(out, "", llog.LstdFlags),
		errLog:  llog.New(err, "", llog.LstdFlags),
	}
}

// Infof logs informational messages
func (l *StdLogger) Infof(format string, v ...interface{}) {
	l.infoLog.Printf(format, v...)
}

// Errorf logs error messages
func (l *StdLogger) Errorf(format string, v ...interface{}) {
	l.errLog.Printf(format, v...)
}

// Debugf logs debug messages
func (l *StdLogger) Debugf(format string, v ...interface{}) {
	l.infoLog.Printf(format, v...)
}

// StructuredLogger writes log messages in JSON.
type StructuredLogger struct {
	logger *zap.SugaredLogger
}

// Infof logs informational messages
func (l *StructuredLogger) Infof(format string, v ...interface{}) {
	l.logger.Infof(format, v...)
}

// Errorf logs error messages
func (l *StructuredLogger) Errorf(format string, v ...interface{}) {
	l.logger.Errorf(format, v...)
}

// Debugf logs debug messages
func (l *StructuredLogger) Debugf(format string, v ...interface{}) {
	l.logger.Infof(format, v...)
}

// NewStructuredLogger creates a Logger that logs messages using JSON.
func NewStructuredLogger() (cloudsql.Logger, func() error) {
	// Configure structured logs to adhere to LogEntry format
	// https://cloud.google.com/logging/docs/reference/v2/rest/v2/LogEntry
	c := zap.NewProductionEncoderConfig()
	c.LevelKey = "severity"
	c.MessageKey = "message"
	c.TimeKey = "timestamp"
	c.EncodeLevel = zapcore.CapitalLevelEncoder
	c.EncodeTime = zapcore.ISO8601TimeEncoder

	enc := zapcore.NewJSONEncoder(c)
	core := zapcore.NewTee(
		zapcore.NewCore(enc, zapcore.Lock(os.Stdout), zap.LevelEnablerFunc(func(l zapcore.Level) bool {
			// Anything below error, goes to the info log.
			return l < zapcore.ErrorLevel
		})),
		zapcore.NewCore(enc, zapcore.Lock(os.Stderr), zap.LevelEnablerFunc(func(l zapcore.Level) bool {
			// Anything at error or higher goes to the error log.
			return l >= zapcore.ErrorLevel
		})),
	)
	l := &StructuredLogger{
		logger: zap.New(core).Sugar(),
	}
	return l, l.logger.Sync
}
