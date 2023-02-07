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

package handler

import (
	"github.com/spf13/pflag"
)

// Options defines configurable handler options.
type Options struct {
	// applicationCredentialRoles sets the roles an application credential
	// is granted on creation.
	applicationCredentialRoles []string
}

// AddFlags adds the options flags to the given flag set.
func (o *Options) AddFlags(f *pflag.FlagSet) {
	f.StringSliceVar(&o.applicationCredentialRoles, "application-credential-roles", nil, "A role to be added to application credentials on creation.  May be specified more than once.")
}
