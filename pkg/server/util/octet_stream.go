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
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// WriteOctetStreamResponse is a generic wrapper for returning a OctetStream payload to the client.
func WriteOctetStreamResponse(w http.ResponseWriter, r *http.Request, code int, body []byte) {
	log := log.FromContext(r.Context())

	w.Header().Add("Content-Type", "application/octet-stream")

	w.WriteHeader(code)

	if _, err := w.Write(body); err != nil {
		log.Error(err, "failed to write response")
	}
}
