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

package errors

import (
	"errors"
)

var (
	// ErrIncorrectArgumentNum is raised when the number of positional parameters
	// are not specified when expected.
	ErrIncorrectArgumentNum = errors.New("incorrect number of arguments specified")

	// ErrInvalidName is raised when a name is zero length or another constraint
	// is invalid.
	ErrInvalidName = errors.New("invalid name specified")

	// ErrInvalidPath is raised when a path is zero length or doesn't exist.
	ErrInvalidPath = errors.New("invalid path specified")

	// ErrInvalidEnvironment is raised when an environment variable is not set.
	ErrInvalidEnvironment = errors.New("invalid environment")

	// ErrNotFound is raised when a requested resource name isn't found.
	ErrNotFound = errors.New("resource name not found")

	// ErrProjectNamespaceUndefined is raised when you try to provision a control
	// plane against a project that hasn't fully provisioned yet.
	ErrProjectNamespaceUndefined = errors.New("project namespace is not set")
)
