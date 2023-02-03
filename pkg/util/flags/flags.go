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
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/api/resource"
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

// QuantityFlag provides parsing and type checking of quanities.
type QuantityFlag struct {
	Quantity *resource.Quantity
}

// Ensure the pflag.Value interface is implemented.
var _ = pflag.Value(&QuantityFlag{})

// String returns the current value.
func (s QuantityFlag) String() string {
	if s.Quantity == nil {
		return ""
	}

	return s.Quantity.String()
}

// Set sets the value and does any error checking.
func (s *QuantityFlag) Set(in string) error {
	quantity, err := resource.ParseQuantity(in)
	if err != nil {
		return err
	}

	s.Quantity = &quantity

	return nil
}

// Type returns the human readable type information.
func (s QuantityFlag) Type() string {
	return "quantity"
}

// StringMapFlag provides a flag that accumulates k/v string pairs.
type StringMapFlag struct {
	Map map[string]string
}

// Ensure the pflag.Value interface is implemented.
var _ = pflag.Value(&StringMapFlag{})

// String returns the current value.
func (s StringMapFlag) String() string {
	//nolint:prealloc
	var pairs []string

	for k, v := range s.Map {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}

	return strings.Join(pairs, ",")
}

// Set sets the value and does any error checking.
func (s *StringMapFlag) Set(in string) error {
	parts := strings.Split(in, "=")
	if len(parts) != 2 {
		return fmt.Errorf("%w: flag requires key=value format", ErrParseFlag)
	}

	if s.Map == nil {
		s.Map = map[string]string{}
	}

	s.Map[parts[0]] = parts[1]

	return nil
}

// Type returns the human readable type information.
func (s StringMapFlag) Type() string {
	return "pair"
}

// DurationFlag provides a flag that parses a Go duration.
type DurationFlag struct {
	Duration time.Duration
}

// String returns the current value.
func (s DurationFlag) String() string {
	return s.Duration.String()
}

// Set sets the value and does any error checking.
func (s *DurationFlag) Set(in string) error {
	duration, err := time.ParseDuration(in)
	if err != nil {
		return err
	}

	s.Duration = duration

	return nil
}

// Type returns the human readable type information.
func (s DurationFlag) Type() string {
	return "duration"
}

// IPNetSliceFlag provides a way to accumulate IP networks.
type IPNetSliceFlag struct {
	IPNetworks []net.IPNet
}

// String returns the current value.
func (s IPNetSliceFlag) String() string {
	l := make([]string, len(s.IPNetworks))

	for i, network := range s.IPNetworks {
		l[i] = network.String()
	}

	return strings.Join(l, ",")
}

// Set sets the value and does any error checking.
func (s *IPNetSliceFlag) Set(in string) error {
	_, n, err := net.ParseCIDR(in)
	if err != nil {
		return err
	}

	s.IPNetworks = append(s.IPNetworks, *n)

	return nil
}

// Type returns the human readable type information.
func (s IPNetSliceFlag) Type() string {
	return "ipNetwork"
}
