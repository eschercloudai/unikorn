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
	"encoding/json"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// WriteJSONResponse is a generic wrapper for returning a JSON payload to the client.
func WriteJSONResponse(w http.ResponseWriter, r *http.Request, code int, response interface{}) {
	log := log.FromContext(r.Context())

	body, err := json.Marshal(response)
	if err != nil {
		log.Error(err, "unable to marshal body")

		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(code)

	if _, err := w.Write(body); err != nil {
		log.Error(err, "failed to write response")
	}
}
