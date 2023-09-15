/*
Copyright 2022-2023 EscherCloud.

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

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/spf13/pflag"

	"github.com/eschercloudai/unikorn/pkg/docs/document"
	"github.com/eschercloudai/unikorn/pkg/docs/formatter"
	"github.com/eschercloudai/unikorn/pkg/util"
	"github.com/eschercloudai/unikorn/pkg/util/trie"
)

var (
	// ErrDocument is raised when the OpenAPI document doesn't meet standards.
	// TODO: push me down into the validation.
	ErrDocument = errors.New("document error")

	// ErrFlag is raised when a flag is invalid.
	ErrFlag = errors.New("flag error")
)

// FormatterVar defines a type that represents a formatter.
type FormatterVar string

const (
	// HTMLFormatter emits raw HTML code, without the need of a full documentation
	// engine checkout.
	HTMLFormatter FormatterVar = "html"

	// MarkdownFormatter emits markdown based code, suitable for things like
	// Docusaurus.
	MarkdownFormatter FormatterVar = "markdown"
)

func (f FormatterVar) String() string {
	return string(f)
}

func (f *FormatterVar) Set(s string) error {
	allowed := []FormatterVar{
		HTMLFormatter,
		MarkdownFormatter,
	}

	ok := false

	for _, a := range allowed {
		if a == FormatterVar(s) {
			ok = true
			break
		}
	}

	if !ok {
		return fmt.Errorf("%w: formatter flag must be one of %v", ErrFlag, allowed)
	}

	*f = FormatterVar(s)

	return nil
}

func (f FormatterVar) Type() string {
	return "string"
}

// options define spell checking options.
type options struct {
	// spellCheck does a spell checking run.
	spellCheck bool

	// openapiSchema defines where the openapi schema lives, may be a relative or
	// absolute path.
	openapiSchema string

	// dictionaries is a list of dictionaries, with one word per line.
	dictionaries []string

	// formatter defines the formatter backend to use.
	formatter FormatterVar

	// output defines where to send the rendered output.
	output string

	// dryRun defines whether to write anything, useful for CI.
	dryRun bool
}

// addFlags adds options to the flag set.
func (o *options) addFlags(flags *pflag.FlagSet) {
	o.formatter = MarkdownFormatter

	flags.BoolVar(&o.spellCheck, "spell-check", true, "Enable spell checking preprocessor")
	flags.StringVar(&o.openapiSchema, "openapi-schema", "pkg/server/openapi/server.spec.yaml", "Path to the openapi schema")
	flags.StringArrayVarP(&o.dictionaries, "dictionary", "d", []string{"/usr/share/dict/british-english", "hack/docs/custom.dict"}, "Path to the dictionary file, may be specified multiple times")
	flags.VarP(&o.formatter, "formatter", "f", "Output formatter type")
	flags.StringVarP(&o.output, "output", "o", "", "Output file")
	flags.BoolVar(&o.dryRun, "dry-run", false, "Whether to run with no side effects")
}

// createSpellChecker initialises our spell checking trie with the
// selected dictionaries.
func createSpellChecker(o *options) (*trie.Trie, error) {
	trie := trie.New()

	for _, dictionary := range o.dictionaries {
		file, err := os.Open(dictionary)
		if err != nil {
			return nil, err
		}

		if err := trie.AddDictionary(file); err != nil {
			return nil, err
		}
	}

	return trie, nil
}

// spellCheckWord checks a single word against the dictionary.
// TODO: We may need to match uris, dates and times.
func spellCheckWord(spellchecker *trie.Trie, word string) bool {
	// Direct match, do this before stripping punctuation as things
	// like e.g. or i.e. will match here.  To be honest, you shouldn't
	// use latin in technical documentation anyway.
	if spellchecker.CheckWord(word) {
		return true
	}

	if _, err := strconv.Atoi(word); err == nil {
		return true
	}

	// Strip out any leading or trailing punctuation or symbols e.g. "'().,
	// and check again.
	for first, firstWidth := utf8.DecodeRuneInString(word); unicode.IsPunct(first) || unicode.IsSymbol(first); first, firstWidth = utf8.DecodeRuneInString(word) {
		word = word[firstWidth:]
	}

	for last, lastWidth := utf8.DecodeLastRuneInString(word); unicode.IsPunct(last) || unicode.IsSymbol(last); last, lastWidth = utf8.DecodeLastRuneInString(word) {
		word = word[:len(word)-lastWidth]
	}

	if spellchecker.CheckWord(word) {
		return true
	}

	first, firstWidth := utf8.DecodeRuneInString(word)

	// If it's capitalised, then make it lower case.
	if unicode.IsUpper(first) {
		if spellchecker.CheckWord(string(unicode.ToLower(first)) + word[firstWidth:]) {
			return true
		}
	}

	return false
}

// spellCheckParagraph takes a blob of text and splits it into tokens
// based on white space, then spell checks the individual words.
func spellCheckParagraph(spellchecker *trie.Trie, paragraph string) error {
	scanner := bufio.NewScanner(bytes.NewBufferString(paragraph))
	scanner.Split(bufio.ScanWords)

	for scanner.Scan() {
		if spellCheckWord(spellchecker, scanner.Text()) {
			continue
		}

		fmt.Println("word not found in dictionary:", scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

// convertParameter does spellchecking and returns the internal representation.
func (o *options) convertParameter(parameter *openapi3.Parameter, spellchecker *trie.Trie) (*document.Parameter, error) {
	if err := spellCheckParagraph(spellchecker, parameter.Description); err != nil {
		return nil, err
	}

	p := &document.Parameter{
		Name:        parameter.Name,
		Description: parameter.Description,
	}

	return p, nil
}

// convertContent does spellchecking and returns the internal representation.
func (o *options) convertContent(content string, media *openapi3.MediaType) *document.Content {
	c := &document.Content{
		Type:    content,
		Example: media.Example,
	}

	if media.Schema != nil {
		c.Schema = media.Schema.Value
	}

	return c
}

// cconvertRequestBod does spellchecking and returns the internal representation.
func (o *options) convertRequestBody(requestBody *openapi3.RequestBody, spellchecker *trie.Trie) (*document.RequestBody, error) {
	if err := spellCheckParagraph(spellchecker, requestBody.Description); err != nil {
		return nil, err
	}

	b := &document.RequestBody{
		Required:    requestBody.Required,
		Description: requestBody.Description,
	}

	for content, media := range requestBody.Content {
		b.Content = o.convertContent(content, media)
	}

	return b, nil
}

// convertResponse does spellchecking and returns the internal representation.
func (o *options) convertResponse(status string, response *openapi3.Response, spellchecker *trie.Trie) (*document.Response, error) {
	if err := spellCheckParagraph(spellchecker, *response.Description); err != nil {
		return nil, err
	}

	r := &document.Response{
		Status:      status,
		Description: *response.Description,
	}

	for content, media := range response.Content {
		r.Content = o.convertContent(content, media)
	}

	return r, nil
}

// convertOperation does spellchecking and returns the internal representation.
func (o *options) convertOperation(method string, operation *openapi3.Operation, spellchecker *trie.Trie) (*document.Operation, error) {
	if err := spellCheckParagraph(spellchecker, operation.Description); err != nil {
		return nil, err
	}

	op := &document.Operation{
		Method:      method,
		Description: operation.Description,
	}

	if operation.RequestBody != nil {
		b, err := o.convertRequestBody(operation.RequestBody.Value, spellchecker)
		if err != nil {
			return nil, err
		}

		op.RequestBody = b
	}

	for status, response := range operation.Responses {
		r, err := o.convertResponse(status, response.Value, spellchecker)
		if err != nil {
			return nil, err
		}

		op.Responses = append(op.Responses, r)
	}

	sort.Stable(op.Responses)

	return op, nil
}

// convertPath does spellchecking and returns the internal representation.
// Additionally it checks that the path is given an API group.
func (o *options) convertPath(path string, pathItem *openapi3.PathItem, spellchecker *trie.Trie) (*document.Path, error) {
	if err := spellCheckParagraph(spellchecker, pathItem.Description); err != nil {
		return nil, err
	}

	groupID, ok := pathItem.Extensions["x-documentation-group"].(string)
	if !ok {
		return nil, fmt.Errorf("%w: path %s must have API group defined", ErrDocument, path)
	}

	p := &document.Path{
		GroupID:     groupID,
		Path:        path,
		Description: pathItem.Description,
	}

	for _, parameter := range pathItem.Parameters {
		pr, err := o.convertParameter(parameter.Value, spellchecker)
		if err != nil {
			return nil, err
		}

		p.Parameters = append(p.Parameters, pr)
	}

	sort.Stable(p.Parameters)

	for method, operation := range pathItem.Operations() {
		op, err := o.convertOperation(method, operation, spellchecker)
		if err != nil {
			return nil, err
		}

		p.Operations = append(p.Operations, op)
	}

	sort.Stable(p.Operations)

	return p, nil
}

// convertGroups ensures the groups extension is implemented and spelling is fine.
func (o *options) convertGroups(doc *openapi3.T, spellchecker *trie.Trie) (document.GroupList, error) {
	data, ok := doc.Extensions["x-documentation-groups"]
	if !ok {
		return nil, fmt.Errorf("%w: document must have API groups defined", ErrDocument)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var groups document.GroupList

	if err := json.Unmarshal(jsonData, &groups); err != nil {
		return nil, err
	}

	for _, group := range groups {
		if err := spellCheckParagraph(spellchecker, group.Name); err != nil {
			return nil, err
		}

		if err := spellCheckParagraph(spellchecker, group.Description); err != nil {
			return nil, err
		}
	}

	return groups, nil
}

// convertDocument takes the raw OpenAPI document and converts it into
// an internal representation that does things like grouping and ordering
// of content.
func (o *options) convertDocument(doc *openapi3.T, spellchecker *trie.Trie) (*document.Document, error) {
	if err := spellCheckParagraph(spellchecker, doc.Info.Description); err != nil {
		return nil, err
	}

	groups, err := o.convertGroups(doc, spellchecker)
	if err != nil {
		return nil, err
	}

	d := &document.Document{
		Name:        doc.Info.Title,
		Description: doc.Info.Description,
		Version:     doc.Info.Version,
		Groups:      groups,
	}

	for path, pathItem := range doc.Paths {
		p, err := o.convertPath(path, pathItem, spellchecker)
		if err != nil {
			return nil, err
		}

		if err := d.Groups.AddPath(p); err != nil {
			return nil, err
		}
	}

	return d, nil
}

// formatSchema recursively does a depth first traversal of the schema, formatting
// each JSON path into a nice table that explains the field.
func formatSchema(schema *openapi3.Schema, f formatter.Formatter, required bool, jsonPath string) {
	// The root element is special and needs defaulting.
	printPath := "."

	if jsonPath != "" {
		printPath = jsonPath
	}

	requiredSymbol := "✘"

	if required {
		requiredSymbol = "✔"
	}

	description := strings.ReplaceAll(schema.Description, "\n", " ")

	f.TD(printPath, schema.Type, requiredSymbol, description)

	switch schema.Type {
	case "object":
		properties := util.Keys(schema.Properties)

		sort.Strings(properties)

		for _, name := range properties {
			property := schema.Properties[name]

			formatSchema(property.Value, f, slices.Contains(schema.Required, name), fmt.Sprintf("%s.%s", jsonPath, name))
		}
	case "array":
		arrayItemRequired := false

		if schema.MinItems > 0 {
			arrayItemRequired = true
		}

		formatSchema(schema.Items.Value, f, arrayItemRequired, fmt.Sprintf("%s.0", jsonPath))
	}
}

// formatParameters emits parameters, if definedi, as a table.
func formatParameters(parameters document.ParameterList, f formatter.Formatter) {
	if len(parameters) == 0 {
		return
	}

	f.Details("Request Parameters", func() {
		f.Table()
		f.TH("Name", "Description")

		for _, p := range parameters {
			f.TD(p.Name, p.Description)
		}

		f.TableEnd()
	})
}

// formatRequestBody emits a details section (hidden by default)
// that includes a description, schema and example.
func formatRequestBody(requestBody *document.RequestBody, f formatter.Formatter) {
	if requestBody == nil {
		return
	}

	f.Details("Request Body", func() {
		f.P(requestBody.Description)

		if requestBody.Content == nil {
			return
		}

		f.P("The content type is \"" + requestBody.Content.Type + "\".")

		if requestBody.Content.Schema != nil {
			f.H5("Fields")
			f.P("This describes the request body object in terms of the JSON path specification that will be familiar to Kubernetes users. Where a child element is an array, it has been substituted with a \"0\", representing the first element indexed from zero.")

			f.Table()
			f.TH("JSON Path", "Type", "Required", "Description")
			formatSchema(requestBody.Content.Schema, f, true, "")
			f.TableEnd()
		}

		if requestBody.Content.Example != nil {
			example, err := json.MarshalIndent(requestBody.Content.Example, "", "  ")
			if err != nil {
				fmt.Println("unable to generate example for request body")
				return
			}

			f.H5("Example")
			f.Code("json", "Example", string(example))
		}
	})
}

// formatRequestBody emits a details section (hidden by default)
// that includes a description, schema and example.
func formatResponse(r *document.Response, f formatter.Formatter) {
	f.Details("HTTP "+r.Status, func() {
		f.P(r.Description)

		if r.Content == nil {
			return
		}

		f.P("The content type is \"" + r.Content.Type + "\".")

		if r.Content.Schema != nil {
			f.H5("Fields")
			f.P("This describes the returned object in terms of the JSON path specification that will be familiar to Kubernetes users. Where a child element is an array, it has been substituted with a \"0\", representing the first element indexed from zero.")

			f.Table()
			f.TH("JSON Path", "Type", "Required", "Description")
			formatSchema(r.Content.Schema, f, true, "")
			f.TableEnd()
		}

		if r.Content.Example != nil {
			example, err := json.MarshalIndent(r.Content.Example, "", "  ")
			if err != nil {
				fmt.Println("unable to generate example for response")
				return
			}

			f.H5("Example")
			f.Code("json", "Example", string(example))
		}
	})
}

// formatOperations emits a set of operations and descriptions, further details
// such as request bodies and responses are hidden away by default as details
// sections.
func formatOperations(operations document.OperationList, f formatter.Formatter) {
	for _, o := range operations {
		f.H4(o.Method)
		f.P(o.Description)

		formatRequestBody(o.RequestBody, f)

		f.Details("Responses", func() {
			for _, r := range o.Responses {
				formatResponse(r, f)
			}
		})
	}
}

// formatPath emits a path, its parameters, and any operations associated with it.
func formatPath(p *document.Path, f formatter.Formatter) {
	f.H3(p.Path)
	f.P(p.Description)

	formatParameters(p.Parameters, f)
	formatOperations(p.Operations, f)
}

// formatGroup emits a proprietary grouping that allows paths to be group together with
// an explainatory description.
func formatGroup(g *document.Group, f formatter.Formatter) {
	f.H2(g.Name)
	f.P(g.Description)

	for _, p := range g.Paths {
		formatPath(p, f)
	}
}

// formatDocument describes the entire API, then all groups of API paths as children.
func formatDocument(d *document.Document, f formatter.Formatter) {
	f.TableOfContentsLevel(2, 3)

	f.H1(d.Name)
	f.P(d.Description)

	f.Warning("This API is currently in beta, and is subject to change without notification.  It is recommended that you use an official client, for example the web console, until general availability.")

	for _, g := range d.Groups {
		formatGroup(g, f)
	}
}

// processDocument walks the document and checks the descriptions are in whatever
// language you have selected, then generates documentation.
func (o *options) processDocument(doc *openapi3.T, spellchecker *trie.Trie, f formatter.Formatter) error {
	d, err := o.convertDocument(doc, spellchecker)
	if err != nil {
		return err
	}

	formatDocument(d, f)

	return nil
}

// run does the main meat, parsing the OpenAPI specification, converting it into an external
// representation and finally rendering the result.
func (o *options) run() error {
	// Load in our dictionaries.
	spellchecker, err := createSpellChecker(o)
	if err != nil {
		return err
	}

	// Load in the OpenAPI schema.
	loader := openapi3.NewLoader()

	doc, err := loader.LoadFromFile("pkg/server/openapi/server.spec.yaml")
	if err != nil {
		return err
	}

	// Create the output file.
	var output io.Writer

	if o.dryRun {
		output = &bytes.Buffer{}
	} else {
		wc, err := os.Create(o.output)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		defer wc.Close()

		output = wc
	}

	// Create the correct formatter.
	var f formatter.Formatter

	switch o.formatter {
	case HTMLFormatter:
		f = formatter.NewHTMLFormatter(output)
	case MarkdownFormatter:
		f = formatter.NewMarkdownFormatter(output)
	}

	// Let's get ready to rumble!
	if err := o.processDocument(doc, spellchecker, f); err != nil {
		return err
	}

	return nil
}

func main() {
	o := &options{}
	o.addFlags(pflag.CommandLine)

	pflag.Parse()

	if err := o.run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
