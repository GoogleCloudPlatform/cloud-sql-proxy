// Copyright 2021 Google LLC

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     https://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"errors"
)

var errSigInt = &exitError{
	Err:  errors.New("SIGINT signal received"),
	Code: 130,
}

var errSigTerm = &exitError{
	Err:  errors.New("SIGINT signal received"),
	Code: 137,
}

// exitError is an error with an exit code, that's returned when the cmd exits.
// When possible, try to match these conventions: https://tldp.org/LDP/abs/html/exitcodes.html
type exitError struct {
	Code int
	Err  error
}

func (e *exitError) Error() string {
	if e.Err == nil {
		return "<missing error>"
	}
	return e.Err.Error()
}
