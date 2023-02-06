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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// natPrefix provides a cache/memoization for GetNATPrefix, be nice to our
// internet brethren.
//
//nolint:gochecknoglobals
var natPrefix string

// GetNATPrefix returns the IP address of the SNAT that the control plane
// and by extension we, sit behind.
func GetNATPrefix(ctx context.Context) (string, error) {
	if natPrefix != "" {
		return natPrefix, nil
	}

	natAddress := struct {
		IP string `json:"ip"`
	}{}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org?format=json", nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{}

	response, err := client.Do(request)
	if err != nil {
		return "", err
	}

	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	if err := json.Unmarshal(body, &natAddress); err != nil {
		return "", err
	}

	natPrefix = fmt.Sprintf("%s/32", natAddress.IP)

	return natPrefix, nil
}
