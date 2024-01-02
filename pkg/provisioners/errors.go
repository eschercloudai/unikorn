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

package provisioners

import (
	"errors"
)

var (
	// ErrYield is raised when a provision/deprovision optation could
	// block for a long time, in particular the bits that wait for apllication
	// available status.  This will trigger a controller to requeue the request.
	// The key things are that workers are unblocked, allowing other reconciles
	// to be triggered, and we can pick up an modifications (e.g. the cluster is
	// gubbed - thanks CAPO - and we can delete it without waiting for 10m as the
	// case used to be in the old world.
	ErrYield = errors.New("controller timeout yield")

	// ErrNotFound is when a resource is not found.
	ErrNotFound = errors.New("resource not found")
)
