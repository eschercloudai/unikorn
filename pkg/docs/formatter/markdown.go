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
	"strings"
)

type MarkdownFormatter struct {
	output io.Writer
}

func NewMarkdownFormatter(output io.Writer) *MarkdownFormatter {
	return &MarkdownFormatter{
		output: output,
	}
}

func (f MarkdownFormatter) H1(a ...any) {
	fmt.Fprint(f.output, "# ")
	fmt.Fprintln(f.output, a...)
	fmt.Fprintln(f.output)
}

func (f MarkdownFormatter) H2(a ...any) {
	fmt.Fprint(f.output, "## ")
	fmt.Fprintln(f.output, a...)
	fmt.Fprintln(f.output)
}

func (f MarkdownFormatter) H3(a ...any) {
	fmt.Fprint(f.output, "### ")
	fmt.Fprintln(f.output, a...)
	fmt.Fprintln(f.output)
}

func (f MarkdownFormatter) H4(a ...any) {
	fmt.Fprint(f.output, "#### ")
	fmt.Fprintln(f.output, a...)
	fmt.Fprintln(f.output)
}

func (f MarkdownFormatter) H5(a ...any) {
	fmt.Fprint(f.output, "##### ")
	fmt.Fprintln(f.output, a...)
	fmt.Fprintln(f.output)
}

func (f MarkdownFormatter) P(a string) {
	fmt.Fprintln(f.output, a)
	fmt.Fprintln(f.output)
}

func (f MarkdownFormatter) B(a string) {
	fmt.Fprint(f.output, "**", a, "**")
}

func (f MarkdownFormatter) Details(summary string, callback func()) {
	fmt.Fprintln(f.output, "<details>")
	fmt.Fprintln(f.output, "<summary>", summary, "</summary>")
	fmt.Fprintln(f.output)
	callback()
	fmt.Fprintln(f.output, "</details>")
	fmt.Fprintln(f.output)
}

func (f MarkdownFormatter) Code(lang string, title string, code string) {
	fmt.Fprintln(f.output, "```", lang, fmt.Sprintf("title=%s", title))
	fmt.Fprintln(f.output, code)
	fmt.Fprintln(f.output, "```")
}

func (f MarkdownFormatter) Table() {
}

func (f MarkdownFormatter) TableEnd() {
}

func (f MarkdownFormatter) TH(a ...string) {
	fmt.Fprint(f.output, "|")

	for _, s := range a {
		fmt.Fprint(f.output, strings.ReplaceAll(s, "\n", " "), "|")
	}

	fmt.Fprintln(f.output)
	fmt.Fprint(f.output, "|")

	for range a {
		fmt.Fprint(f.output, "---|")
	}

	fmt.Fprintln(f.output)
}

func (f MarkdownFormatter) TD(a ...string) {
	fmt.Fprint(f.output, "|")

	for _, s := range a {
		fmt.Fprint(f.output, strings.ReplaceAll(s, "\n", " "), "|")
	}

	fmt.Fprintln(f.output)
}

func (f MarkdownFormatter) TableOfContentsLevel(min int, max int) {
	fmt.Fprintln(f.output, "---")
	fmt.Fprintln(f.output, "toc_min_heading_level:", min)
	fmt.Fprintln(f.output, "toc_max_heading_level:", max)
	fmt.Fprintln(f.output, "---")
	fmt.Fprintln(f.output)
}

func (f MarkdownFormatter) Warning(description string) {
	fmt.Fprintln(f.output, ":::caution")
	fmt.Fprintln(f.output)
	fmt.Fprintln(f.output, description)
	fmt.Fprintln(f.output)
	fmt.Fprintln(f.output, ":::")
	fmt.Fprintln(f.output)
}
