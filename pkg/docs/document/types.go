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

package document

import (
	"github.com/getkin/kin-openapi/openapi3"
)

type Content struct {
	Type    string
	Example interface{}
	Schema  *openapi3.Schema
}

type Parameter struct {
	Name        string
	Description string
}

type ParameterList []*Parameter

type RequestBody struct {
	Required    bool
	Description string
	Content     *Content
}

type Response struct {
	Status      string
	Description string
	Content     *Content
}

type ResponseList []*Response

type Operation struct {
	Method      string
	Description string
	RequestBody *RequestBody
	Responses   ResponseList
}

type OperationList []*Operation

type Path struct {
	GroupID     string
	Path        string
	Description string
	Parameters  ParameterList
	Operations  OperationList
}

type PathList []*Path

type Group struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Paths       PathList
}

type GroupList []*Group

type Document struct {
	Name        string
	Description string
	Version     string
	Groups      GroupList
}
