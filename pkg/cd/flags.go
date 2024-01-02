/*
Copyright 2022-2024 EscherCloud.

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

package cd

import (
	"slices"

	"github.com/spf13/pflag"

	"github.com/eschercloudai/unikorn/pkg/errors"
)

// DriverKindFlag wraps up the driver kind in a flag that can be used on the CLI.
type DriverKindFlag struct {
	Kind DriverKind
}

var _ pflag.Value = &DriverKindFlag{}

// String implemenets the pflag.Value interface.
func (s *DriverKindFlag) String() string {
	return string(s.Kind)
}

// Set implemenets the pflag.Value interface.
func (s *DriverKindFlag) Set(in string) error {
	valid := []DriverKind{
		DriverKindArgoCD,
	}

	value := DriverKind(in)

	if !slices.Contains(valid, value) {
		return errors.ErrParseFlag
	}

	s.Kind = value

	return nil
}

// Type implemenets the pflag.Value interface.
func (s *DriverKindFlag) Type() string {
	return "string"
}
