// Copyright 2021 Google LLC
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

package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/logging"
	"github.com/spf13/cobra"
)

var errTermSignal = fmt.Errorf("SIGINT or SIGTERM received")

// New returns a *cobra.Command object representing the proxy.
func New() *cobra.Command {
	return &cobra.Command{
		Use:   "cloud_sql_proxy instance_connection_name...",
		Short: "cloud_sql_proxy provides a secure way to authorize connections to Cloud SQL.",
		Long: `The Cloud SQL Auth proxy provides IAM-based authorization and encryption when
connecting to a Cloud SQL instance. It listens on a local port and forwards connections
to your instance's IP address, providing a secure connection without having to manage
any client SSL certificates.`,
		Run: runWrapper,
	}
}

func runWrapper(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownCh := make(chan error)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		select {
		case <-signals:
		case <-cmd.Context().Done():
			// This scenario should only happen in tests when the context is set
		}
		shutdownCh <- errTermSignal
	}()

	go runProxy(ctx, cmd, args, shutdownCh)

	err := <-shutdownCh
	switch {
	case errors.Is(err, errTermSignal):
		cmd.Println("TERM signal received. Shutting down...")
		os.Exit(0)
	default:
		logging.Errorf("Shutting down due to error: %s", err)
		os.Exit(1)
	}
}

func runProxy(ctx context.Context, cmd *cobra.Command, args []string, shutdownCh chan error) {
	dialer, err := cloudsqlconn.NewDialer(ctx)
	if err != nil {
		shutdownCh <- fmt.Errorf("could not initialize dialer: %v", err)
	}

	port := 5000 // TODO: figure out better port allocation strategy
	mnt := make([]*socketMount, 0, len(args))
	for _, i := range args {
		m := newSocketMount(*dialer, i)
		addr, err := m.Listen(ctx, "tcp4", net.JoinHostPort("", fmt.Sprint(port)))
		if err != nil {
			shutdownCh <- err
			return
		}
		cmd.Printf("[%s] Listening on %s\n", i, addr.String())
		mnt = append(mnt, m)
	}
	for _, m := range mnt {
		go func(mnt *socketMount) {
			err := mnt.Serve(ctx)
			if err != nil {
				shutdownCh <- err
			}
		}(m)
	}

	cmd.Println("Ready for new connections!")
}
