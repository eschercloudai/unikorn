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

package util

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// GetURLCACertificate works out the CA certificate for a HTTPS endpoint.
func GetURLCACertificate(host string) ([]byte, error) {
	authURL, err := url.Parse(host)
	if err != nil {
		return nil, err
	}

	conn, err := tls.Dial("tcp", authURL.Host, nil)
	if err != nil {
		return nil, err
	}

	defer conn.Close()

	chains := conn.ConnectionState().VerifiedChains
	chain := chains[0]
	ca := chain[len(chain)-1]

	pemBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.Raw,
	}

	return pem.EncodeToMemory(pemBlock), nil
}

// Filter is a generic filter function that returns a new filtered slice.
func Filter[T any](l []T, callback func(T) bool) []T {
	var r []T

	for _, i := range l {
		if callback(i) {
			r = append(r, i)
		}
	}

	return r
}
