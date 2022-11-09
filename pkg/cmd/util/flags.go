/*
Copyright 2022 EscherCloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/spf13/pflag"
)

var (
	// ErrParseFlag is raised when flag parsing fails.
	ErrParseFlag = errors.New("flag was unable to be parsed")
)

// SemverFlag provides parsing and type checking of semantic versions.
type SemverFlag struct {
	// Semver specifies a default if set, and can be overridden by
	// a call to Set().
	Semver string
}

// Ensure the pflag.Value interface is implemented.
var _ = pflag.Value(&SemverFlag{})

// String returns the current value.
func (s SemverFlag) String() string {
	return s.Semver
}

// Set sets the value and does any error checking.
func (s *SemverFlag) Set(in string) error {
	ok, err := regexp.MatchString(`^v(?:[0-9]+\.){2}(?:[0-9]+)$`, in)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("%w: flag must match v1.2.3", ErrParseFlag)
	}

	s.Semver = in

	return nil
}

// Type returns the human readable type information.
func (s SemverFlag) Type() string {
	return "semver"
}
