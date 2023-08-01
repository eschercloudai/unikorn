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

package server

import (
	"time"

	"github.com/spf13/pflag"
)

// Options allows server options to be overridden.
type Options struct {
	// ListenAddress tells the server what to listen on, you shouldn't
	// need to change this, its already non-privileged and the default
	// should be modified to avoid clashes with other services e.g prometheus.
	ListenAddress string

	// ReadTimeout defines how long before we give up on the client,
	// this should be fairly short.
	ReadTimeout time.Duration

	// ReadHeaderTimeout defines how long before we give up on the client,
	// this should be fairly short.
	ReadHeaderTimeout time.Duration

	// WriteTimeout defines how long we take to respond before we give up.
	// Ideally we'd like this to be short, but Openstack in general sucks
	// for performance.  Additionally some calls like cluster creation can
	// do a cascading create, e.g. create a default control plane, than in
	// turn creates a project.
	WriteTimeout time.Duration

	// OTLPEndpoint defines whether to ship spans to an OTLP consumer or
	// not, and where to send them to.
	OTLPEndpoint string

	// RequestTimeout places a hard limit on all requests lengths.
	RequestTimeout time.Duration
}

// addFlags allows server options to be modified.
func (o *Options) AddFlags(f *pflag.FlagSet) {
	f.StringVar(&o.ListenAddress, "server-listen-address", ":6080", "API listener address.")
	f.DurationVar(&o.ReadTimeout, "server-read-timeout", time.Second, "How long to wait for the client to send the request body.")
	f.DurationVar(&o.ReadHeaderTimeout, "server-read-header-timeout", time.Second, "How long to wait for the client to send headers.")
	f.DurationVar(&o.WriteTimeout, "server-write-timeout", 10*time.Second, "How long to wait for the API to respond to the client.")
	f.DurationVar(&o.RequestTimeout, "server-request-timeout", 30*time.Second, "How long to wait of a request to be serviced.")
	f.StringVar(&o.OTLPEndpoint, "otlp-endpoint", "", "An optional OTLP endpoint to ship spans to.")
}
