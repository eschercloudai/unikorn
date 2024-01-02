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

import (
	"fmt"
	"io"
)

type HTMLFormatter struct {
	output io.Writer
}

func NewHTMLFormatter(output io.Writer) *HTMLFormatter {
	return &HTMLFormatter{
		output: output,
	}
}

func (f HTMLFormatter) H1(a ...any) {
	fmt.Fprint(f.output, "<h1>")
	fmt.Fprint(f.output, injectSpaces(a)...)
	fmt.Fprintln(f.output, "</h1>")
	fmt.Fprintln(f.output)
}

func (f HTMLFormatter) H2(a ...any) {
	fmt.Fprint(f.output, "<h2>")
	fmt.Fprint(f.output, injectSpaces(a)...)
	fmt.Fprintln(f.output, "</h2>")
	fmt.Fprintln(f.output)
}

func (f HTMLFormatter) H3(a ...any) {
	fmt.Fprint(f.output, "<h3>")
	fmt.Fprint(f.output, injectSpaces(a)...)
	fmt.Fprintln(f.output, "</h3>")
	fmt.Fprintln(f.output)
}

func (f HTMLFormatter) H4(a ...any) {
	fmt.Fprint(f.output, "<h4>")
	fmt.Fprint(f.output, injectSpaces(a)...)
	fmt.Fprintln(f.output, "</h4>")
	fmt.Fprintln(f.output)
}

func (f HTMLFormatter) H5(a ...any) {
	fmt.Fprint(f.output, "<h5>")
	fmt.Fprint(f.output, injectSpaces(a)...)
	fmt.Fprintln(f.output, "</h5>")
	fmt.Fprintln(f.output)
}

func (f HTMLFormatter) P(a string) {
	fmt.Fprintln(f.output, "<p>")
	fmt.Fprintln(f.output, a)
	fmt.Fprintln(f.output, "</p>")
	fmt.Fprintln(f.output)
}

func (f HTMLFormatter) Details(summary string, callback func()) {
	fmt.Fprintln(f.output, "<details>")
	fmt.Fprintln(f.output, "<summary>", summary, "</summary>")
	callback()
	fmt.Fprintln(f.output, "</details>")
	fmt.Fprintln(f.output)
}

func (f HTMLFormatter) Code(_ string, title string, code string) {
	fmt.Fprintln(f.output, "<b>", title, "</b>")
	fmt.Fprintln(f.output, "<code>")
	fmt.Fprintln(f.output, code)
	fmt.Fprintln(f.output, "</code>")
}

func (f HTMLFormatter) Table() {
	fmt.Fprintln(f.output, "<table>")
}

func (f HTMLFormatter) TableEnd() {
	fmt.Fprintln(f.output, "</table>")
}

func (f HTMLFormatter) TH(a ...string) {
	fmt.Fprintln(f.output, "<tr>")

	for _, s := range a {
		fmt.Fprintln(f.output, "<th>", s, "</th>")
	}

	fmt.Fprintln(f.output, "</tr>")
}

func (f HTMLFormatter) TD(a ...string) {
	fmt.Fprintln(f.output, "<tr>")

	for _, s := range a {
		fmt.Fprintln(f.output, "<td>", s, "</td>")
	}

	fmt.Fprintln(f.output, "</tr>")
}

func (f HTMLFormatter) TableOfContentsLevel(int, int) {
}

func (f HTMLFormatter) Warning(description string) {
	fmt.Fprintln(f.output, `<div class="admonition warning">`)
	f.H3("Warning")
	fmt.Fprintln(f.output)
	fmt.Fprintln(f.output, description)
	fmt.Fprintln(f.output)
}
