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

package util

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

// ToPointer is usually used in tests to get a pointer to a const.
func ToPointer[T any](t T) *T {
	return &t
}

// Keys returns the keys from a string map.
//
//nolint:prealloc
func Keys[T any](m map[string]T) []string {
	var r []string

	for k := range m {
		r = append(r, k)
	}

	return r
}

var (
	ErrK8SConnectionError = errors.New("unable to connection the kubernetes API")
)

type DefaultK8SAPITester struct{}

func (t *DefaultK8SAPITester) Connect(ctx context.Context, config *clientcmdapi.Config) error {
	restConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return err
	}

	c, err := client.New(restConfig, client.Options{})
	if err != nil {
		return err
	}

	var svc corev1.Service

	if err := c.Get(ctx, client.ObjectKey{Namespace: "default", Name: "kubernetes"}, &svc); err != nil {
		return ErrK8SConnectionError
	}

	return nil
}
