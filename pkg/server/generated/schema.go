// Package generated provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen version v1.12.4 DO NOT EDIT.
package generated

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// Base64 encoded, gzipped, json marshaled Swagger object
var swaggerSpec = []string{

	"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",
}

// GetSwagger returns the content of the embedded swagger specification file
// or error if failed to decode
func decodeSpec() ([]byte, error) {
	zipped, err := base64.StdEncoding.DecodeString(strings.Join(swaggerSpec, ""))
	if err != nil {
		return nil, fmt.Errorf("error base64 decoding spec: %s", err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(zipped))
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}
	var buf bytes.Buffer
	_, err = buf.ReadFrom(zr)
	if err != nil {
		return nil, fmt.Errorf("error decompressing spec: %s", err)
	}

	return buf.Bytes(), nil
}

var rawSpec = decodeSpecCached()

// a naive cached of a decoded swagger spec
func decodeSpecCached() func() ([]byte, error) {
	data, err := decodeSpec()
	return func() ([]byte, error) {
		return data, err
	}
}

// Constructs a synthetic filesystem for resolving external references when loading openapi specifications.
func PathToRawSpec(pathToFile string) map[string]func() ([]byte, error) {
	var res = make(map[string]func() ([]byte, error))
	if len(pathToFile) > 0 {
		res[pathToFile] = rawSpec
	}

	return res
}

// GetSwagger returns the Swagger specification corresponding to the generated code
// in this file. The external references of Swagger specification are resolved.
// The logic of resolving external references is tightly connected to "import-mapping" feature.
// Externally referenced files must be embedded in the corresponding golang packages.
// Urls can be supported but this task was out of the scope.
func GetSwagger() (swagger *openapi3.T, err error) {
	var resolvePath = PathToRawSpec("")

	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	loader.ReadFromURIFunc = func(loader *openapi3.Loader, url *url.URL) ([]byte, error) {
		var pathToFile = url.String()
		pathToFile = path.Clean(pathToFile)
		getSpec, ok := resolvePath[pathToFile]
		if !ok {
			err1 := fmt.Errorf("path not found: %s", pathToFile)
			return nil, err1
		}
		return getSpec()
	}
	var specData []byte
	specData, err = rawSpec()
	if err != nil {
		return
	}
	swagger, err = loader.LoadFromData(specData)
	if err != nil {
		return
	}
	return
}
