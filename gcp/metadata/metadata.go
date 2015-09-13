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

// Package metadata implements the dialog with the Google Compute
// Engine metadata server.
package metadata

import (
	"encoding/json"
	"fmt"

	md "google.golang.org/cloud/compute/metadata"
)

const retries = 5

type lazyData func() (string, error)
type getter func(s string) (string, error)

// Init will set the destination string to the result of the lazyData function iff
// *dst == "".
func (d lazyData) Init(dst *string) (err error) {
	if *dst == "" {
		*dst, err = d()
	}
	return err
}

// JSON unmarshals the result of the lazyData function into the destination variable.
func (d lazyData) JSON(dst interface{}) error {
	str, err := d()
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(str), dst); err != nil {
		return fmt.Errorf("metadata: can't unmarshal %q: %v", str, err)
	}
	return err
}

// metadata returns a function which can be used to read the metadata value at
// the provided path.
func metadata(path string) lazyData {
	return metadataWithGet(path, md.Get)
}

func metadataWithGet(path string, get getter) lazyData {
	return func() (string, error) {
		var res string
		var err error
		for r := retries; r > 0; r-- {
			if res, err = get(path); err == nil {
				return res, nil
			}
		}

		return "", fmt.Errorf("metadata(%q): %v", path, err)
	}
}

// AccessToken retrieves an OAuth2 Access Token from the metadata
// server. The result is raw JSON.
// Example:
// {"access_token":"ya29.TNSOBND6jlX61DODJJJERKY5Jsa-MFiGOldi3odisjap1uJP3ifjodkNS2KAg","expires_in":2768,"token_type":"Bearer"}
var AccessToken = metadata("instance/service-accounts/default/token")
