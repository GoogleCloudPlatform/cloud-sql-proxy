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
	"os"
	"os/signal"
	"syscall"

	"github.com/GoogleCloudPlatform/cloudsql-proxy/v2/internal/proxy"
	"github.com/spf13/cobra"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := NewCommand().Execute(); err != nil {
		exit := 1
		if terr, ok := err.(*exitError); ok {
			exit = terr.Code
		}
		os.Exit(exit)
	}
}

// Command represents an invocation of the Cloud SQL Auth Proxy.
type Command struct {
	*cobra.Command
	conf *proxy.Config
}

// NewCommand returns a Command object representing an invocation of the proxy.
func NewCommand() *Command {
	c := &Command{}
	conf := &proxy.Config{}
	ccmd := &cobra.Command{
		Use:   "cloud_sql_proxy instance_connection_name...",
		Short: "cloud_sql_proxy provides a secure way to authorize connections to Cloud SQL.",
		Long: `The Cloud SQL Auth proxy provides IAM-based authorization and encryption when
connecting to Cloud SQL instances. It listens on a local port and forwards connections
to your instance's IP address, providing a secure connection without having to manage
any client SSL certificates.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSignalWrapper(c, args)
		},
	}
	ccmd.PersistentFlags().StringVarP(&conf.Addr, "address", "a", "127.0.0.1",
		"Address on which to bind Cloud SQL instance listeners.")

	c.conf = conf
	c.Command = ccmd
	return c
}

// runSignalWrapper watches for SIGTERM and SIGINT and interupts execution if necessary.
func runSignalWrapper(cmd *Command, args []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	shutdownCh := make(chan error)

	// watch for sigterm / sigint signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		var s os.Signal
		select {
		case s = <-signals:
		case <-cmd.Context().Done():
			// this should only happen when the context supplied in tests in canceled
			s = syscall.SIGINT
		}
		switch s {
		case syscall.SIGINT:
			shutdownCh <- errSigInt
		case syscall.SIGTERM:
			shutdownCh <- errSigTerm
		}
	}()

	// Start the proxy asynchronously, so we can exit early if a shutdown signal is sent
	startCh := make(chan *proxy.Client)
	go func() {
		defer close(startCh)
		p, err := proxy.NewClient(ctx, cmd.Command, cmd.conf, args)
		if err != nil {
			shutdownCh <- fmt.Errorf("unable to start: %v", err)
			return
		}
		startCh <- p
	}()
	// Wait for either startup to finish or a signal to interupt
	var p *proxy.Client
	select {
	case err := <-shutdownCh:
		return err
	case p = <-startCh:
	}
	cmd.Println("The proxy has started successfully and is ready for new connections!")
	defer p.Close()

	go func() {
		shutdownCh <- p.Serve(ctx)
	}()

	err := <-shutdownCh
	switch {
	case errors.Is(err, errSigInt):
		cmd.PrintErrln("SIGINT signal received. Shuting down...")
	case errors.Is(err, errSigTerm):
		cmd.PrintErrln("SIGTERM signal received. Shuting down...")
	default:
		cmd.PrintErrf("The proxy has encountered a terminal error: %v\n", err)
	}
	return err
}
