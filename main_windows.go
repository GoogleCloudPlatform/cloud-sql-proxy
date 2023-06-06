// Copyright 2023 Google LLC
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
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/cmd"
	"github.com/GoogleCloudPlatform/cloud-sql-proxy/v2/internal/log"
	"golang.org/x/sys/windows/svc"
	"gopkg.in/natefinch/lumberjack.v2"
)

type windowsService struct{}

func (m *windowsService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	// start the service
	changes <- svc.Status{State: svc.StartPending}

	// set up the log file
	exePath, err := os.Executable()
	if err != nil {
		changes <- svc.Status{State: svc.StopPending}
		return true, 101 // service specific exit code=101
	}

	logFolder := filepath.Join(filepath.Dir(exePath), "logs")
	os.Mkdir(logFolder, 0644) // ignore all errors

	logFile := &lumberjack.Logger{
		Filename:   filepath.Join(logFolder, "cloud-sql-proxy.log"),
		MaxSize:    50, // megabytes
		MaxBackups: 10,
		MaxAge:     30, //days
	}

	logger := log.NewStdLogger(logFile, logFile)
	logger.Infof("Starting...")

	// start the main command
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := cmd.NewCommand(cmd.WithLogger(logger))

	cmdErrCh := make(chan error, 1)
	go func() {
		cmdErrCh <- app.ExecuteContext(ctx)
	}()

	// now running
	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	var cmdErr error

loop:
	for {
		select {
		case err := <-cmdErrCh:
			cmdErr = err
			break loop

		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				// testing deadlock from https://code.google.com/archive/p/winsvc/issues/4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus

			case svc.Stop, svc.Shutdown:
				cancel()

			default:
				logger.Errorf("unexpected control request #%d", c)
			}
		}
	}

	// start shutting down
	logger.Infof("Stopping...")

	changes <- svc.Status{State: svc.StopPending}

	if cmdErr != nil && errors.Is(cmdErr, context.Canceled) {
		logger.Errorf("Unexpected error: %v", cmdErr)
		return true, 2
	}

	return false, 0
}

func main() {
	// determine if running as a windows service
	inService, err := svc.IsWindowsService()
	if err != nil {
		os.Exit(99) // failed to determine service status
	}

	// running as service?
	if inService {
		err := svc.Run("cloud-sql-proxy", &windowsService{})
		if err != nil {
			os.Exit(100) // failed to execute service
		}
		return
	}

	// run as commandline
	cmd.Execute()
}
