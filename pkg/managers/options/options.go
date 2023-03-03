/*
Copyright 2022-2023 EscherCloud.

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

package options

import (
	"github.com/spf13/pflag"
)

// Options defines common controller options.
type Options struct {
	// MaxConcurrentReconciles allows requests to be processed
	// concurrently.  Be warned, this will inrcrease memory utilization
	// and may need to update the Helm limits.
	MaxConcurrentReconciles int
}

func (o *Options) AddFlags(flags *pflag.FlagSet) {
	flags.IntVar(&o.MaxConcurrentReconciles, "--max-concurrency", 16, "Maximum number of requests to process at the same time")
}
