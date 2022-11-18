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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	// ErrParseFlag is raised when flag parsing fails.
	ErrParseFlag = errors.New("flag was unable to be parsed")
)

// RequiredVar registers a generic flag marked as required.
func RequiredVar(cmd *cobra.Command, p pflag.Value, name, usage string) {
	cmd.Flags().Var(p, name, usage)

	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(err)
	}
}

// RequiredStringVarWithCompletion registers a string flag marked as required and
// with a completion function.
func RequiredStringVarWithCompletion(cmd *cobra.Command, p *string, name, value, usage string, f func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)) {
	cmd.Flags().StringVar(p, name, value, usage)

	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(err)
	}

	if err := cmd.RegisterFlagCompletionFunc(name, f); err != nil {
		panic(err)
	}
}

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
