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
	"strings"
)

// SplitYAML takes a yaml manifest and splits it into individual objects
// discarding any empty sections.
func SplitYAML(s string) []string {
	sections := strings.Split(s, "\n---\n")

	var yamls []string

	// Discard any empty sections.
	for _, section := range sections {
		if strings.TrimSpace(section) != "" {
			yamls = append(yamls, section)
		}
	}

	return yamls
}
