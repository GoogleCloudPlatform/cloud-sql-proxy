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

// Package prettyprint contains methods that improve logging.
package prettyprint

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"strings"
)

// CancelTripper represents a RoundTripper that supports timeouts via
// CancelRequest.
type CancelTripper interface {
	http.RoundTripper

	CancelRequest(*http.Request)
}

// LoggingTransport wraps an http.RoundTripper and dumps out the requests
// and responses that the RoundTripper makes. InfoFunc and ErrorFunc should be
// set to the format string functions that should be used to output, e.g.
// functions like fmt.Printf/fmt.Errorf, t.Infof/t.Errorf.
type LoggingTransport struct {
	Logf func(format string, args ...interface{})
	CancelTripper
}

var httpDump = struct {
	request  func(r *http.Request, body bool) ([]byte, error)
	response func(r *http.Response, body bool) ([]byte, error)
}{
	request:  httputil.DumpRequest,
	response: httputil.DumpResponse,
}

func (t *LoggingTransport) logRequest(id int64, req *http.Request) {
	if t.Logf == nil {
		return
	}
	dump, err := httpDump.request(req, true)

	if err != nil {
		t.Logf("[%X] DumpRequest(%v, %v) error: %v", id, req, true, err)
	} else {
		t.Logf("[%X] Sending request:\n%s", id, Bytes(dump))
	}
}

func (t *LoggingTransport) logResponse(id int64, resp *http.Response) {
	if t.Logf == nil {
		return
	}
	dump, err := httpDump.response(resp, true)

	if err != nil {
		t.Logf("[%X] DumpResponse(%v, %v) error: %v", id, resp, true, err)
	} else {
		t.Logf("[%X] Got response:\n%s", id, Bytes(dump))
	}
}

// RoundTrip wraps the Transport's own RoundTrip function to dump requests
// and responses.
func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// During interleaving RoundTrips it's hard to tell which request goes with which response.
	// Printing this ID with both the request and response helps disambiguate.
	id := rand.Int63()

	t.logRequest(id, req)
	resp, err := t.CancelTripper.RoundTrip(req)
	if err != nil {
		if t.Logf != nil {
			t.Logf("[%X] RoundTrip error: %v", id, err)
		}
		return nil, err
	}
	t.logResponse(id, resp)

	return resp, nil
}

// String surrounds a multi-line string with ASCII characters. No
// end-of-line is added at the end. This is useful for making
// multi-line string show up better in when printed using the log
// functions.
func String(s string) string {
	body := strings.Replace(s, "\n", "\n|", -1)
	return fmt.Sprintf("+--- START ---\n|%s\n+--- END ---", body)
}

// Bytes is the same as PrettyPrintString but for a slice of bytes.
func Bytes(b []byte) string {
	return String(string(b))
}

func printRow(r map[string]string, columns []string, values []interface{}) {
	for i, value := range values {
		switch v := value.(type) {
		case nil:
			r[columns[i]] = "NULL"
		case []byte:
			r[columns[i]] = string(v)
		default:
			r[columns[i]] = fmt.Sprint(v)
		}
	}
}

// SQLRows returns a stringified version of the rows passed in,
// keyed by the column name. This will Close() the rows object too.
func SQLRows(rows *sql.Rows) (map[string]string, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("rows.Columns() error: %v", err)
	}

	values := make([]interface{}, len(columns))
	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	fRows := make(map[string]string)
	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, fmt.Errorf("rows.Scan() error: %v", err)
		}
		printRow(fRows, columns, values)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return fRows, nil
}

// Any returns a stringified version of parameter.
func Any(d interface{}) string {
	b, err := json.Marshal(d)
	if err != nil {
		return err.Error()
	}
	return string(b)
}
