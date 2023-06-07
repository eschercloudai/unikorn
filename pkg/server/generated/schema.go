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
