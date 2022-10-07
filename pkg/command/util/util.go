package util

import (
	"strings"
	"text/template"
)

// TemplatedString allows dynamic templating e.g. variable expansion, of
// strings, typically in help text examples.
func TemplatedString(s string, data any) string {
	t := template.New("root")

	t, err := t.Parse(s)
	if err != nil {
		panic(err)
	}

	out := &strings.Builder{}

	if err := t.Execute(out, data); err != nil {
		panic(err)
	}

	return out.String()
}
