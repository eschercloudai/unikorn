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

package formatter

//nolint:interfacebloat
type Formatter interface {
	// These are standard HTML types of markup.
	H1(...any)
	H2(...any)
	H3(...any)
	H4(...any)
	H5(...any)
	P(string)
	Details(string, func())
	Code(string, string, string)
	Table()
	TableEnd()
	TH(...string)
	TD(...string)

	// These are more specialised mark up types e.g.
	// admonitions.
	TableOfContentsLevel(int, int)
	Warning(description string)
}
