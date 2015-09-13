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

package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strconv"

	"log"
)

// Conn represents a connection from a client to a specific instance.
type Conn struct {
	Instance string
	Conn     net.Conn
}

// CertSource is how a Client obtains various certificates required for operation.
type CertSource interface {
	// Local returns a certificate that can be used to authenticate with the
	// provided instance.
	Local(instance string) (tls.Certificate, error)
	// Remote returns the instance's CA certificate and address.
	// If cert is nil or addr is "" the instance may not support connecting via SSL.
	// If addr is "" the instance may not exist.
	Remote(instance string) (cert *x509.Certificate, addr string, err error)
}

// Client is a type to handle connecting to a Server. All fields are required
// unless otherwise specified.
type Client struct {
	// Port designates which remote port should be used when connecting to
	// instances. This value is defined by the server-side code, but for now it
	// should always be 3307.
	Port  int
	Certs CertSource
	Conns *ConnSet
	// Dialer should return a new connection to the provided address. It is
	// called on each new connection to an instance. net.Dial will be used if
	// left nil.
	Dialer func(net, addr string) (net.Conn, error)
}

// Run causes the client to start waiting for new connections to connSrc and
// proxy them to the destination instance. It blocks until connSrc is closed.
func (c Client) Run(connSrc <-chan Conn) {
	for conn := range connSrc {
		go c.handleConn(conn)
	}

	if err := c.Conns.Close(); err != nil {
		log.Printf("closing client had error: %v", err)
	}
}

func (c Client) handleConn(conn Conn) {
	server, err := c.Dial(conn.Instance)
	if err != nil {
		log.Printf("couldn't connect to %q: %v", conn.Instance, err)
		conn.Conn.Close()
		return
	}

	if false {
		// Log the connection's traffic via the debug connection if we're in a
		// verbose mode. Note that this is the unencrypted traffic stream.
		conn.Conn = dbgConn{conn.Conn}
	}

	suffix := fmt.Sprintf(" %q via %s", conn.Instance, server.RemoteAddr())
	c.Conns.Add(conn.Instance, conn.Conn)
	go copyThenClose("to"+suffix, server, conn.Conn)
	copyThenClose("from"+suffix, conn.Conn, server)
	if err := c.Conns.Remove(conn.Instance, conn.Conn); err != nil {
		log.Print(err)
	}
}

// Dial uses the configuration stored in the client to connect to an instance.
// If this func returns a nil error the connection is correctly authenticated
// to connect to the instance.
func (c Client) Dial(instance string) (net.Conn, error) {
	mycert, err := c.Certs.Local(instance)
	if err != nil {
		return nil, err
	}

	scert, addr, err := c.Certs.Remote(instance)
	if err != nil {
		return nil, err
	}
	addr += ":" + strconv.Itoa(c.Port)
	certs := x509.NewCertPool()
	certs.AddCert(scert)

	cconf := &tls.Config{
		ServerName:   instance,
		Certificates: []tls.Certificate{mycert},
		RootCAs:      certs,
	}
	d := c.Dialer
	if d == nil {
		d = net.Dial
	}
	conn, err := d("tcp", addr)
	if err != nil {
		return nil, err
	}
	ret := tls.Client(conn, cconf)
	if err := ret.Handshake(); err != nil {
		ret.Close()
		return nil, err
	}
	return ret, nil
}

// NewConnSrc returns a chan which can be used to receive connections
// on the passed Listener. All requests sent to the returned chan will have the
// instance name provided here. The chan will be closed if the Listener returns
// an error.
func NewConnSrc(instance string, l net.Listener) <-chan Conn {
	ch := make(chan Conn)
	go func() {
		for {
			c, err := l.Accept()
			log.Printf("New connection: %v", c)
			if err != nil {
				log.Printf("listener (%#v) had error: %v", l, err)
				l.Close()
				close(ch)
				return
			}
			ch <- Conn{instance, c}
		}
	}()
	return ch
}
