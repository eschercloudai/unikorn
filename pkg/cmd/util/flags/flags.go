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

package flags

import (
	"errors"

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

// RequireStringdVar registers a string flag marked as required.
func RequiredStringVar(cmd *cobra.Command, p *string, name, value, usage string) {
	cmd.Flags().StringVar(p, name, value, usage)

	if err := cmd.MarkFlagRequired(name); err != nil {
		panic(err)
	}
}

// StringVarWithCompletion registers a string flag with a completion function.
func StringVarWithCompletion(cmd *cobra.Command, p *string, name, value, usage string, f func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)) {
	cmd.Flags().StringVar(p, name, value, usage)

	if err := cmd.RegisterFlagCompletionFunc(name, f); err != nil {
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
