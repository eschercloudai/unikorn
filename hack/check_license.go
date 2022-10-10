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

package main

import (
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

const (
	// goApache2LicenseHeader is an exact match for a license header.
	// TODO: may want to make this fuzzy (e.g. the date) and make it
	// a regex match.
	goApache2LicenseHeader = `/*
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
*/`
)

var (
	// errFail tells you there was an error detected.
	errFail = errors.New("errors detected")

	// errNoComments tells you you've not commented anything, bad engineer.
	errNoComments = errors.New("file contains no comments")

	// errFirstCommentNotLicense tells you that the first comment isn't a license.
	errFirstCommentNotLicense = errors.New("first comment not a valid license")
)

// glob does a recursive walk of the working directory, returning all files that
// match the provided extension e.g. ".go".
func glob(extension string) ([]string, error) {
	var files []string

	appendFileWithExtension := func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			return nil
		}

		if filepath.Ext(path) != extension {
			return nil
		}

		files = append(files, path)

		return nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	if err := filepath.Walk(wd, appendFileWithExtension); err != nil {
		return nil, err
	}

	return files, nil
}

// checkGoLicenseFile parses a source file and checks there is a license header in there.
func checkGoLicenseFile(path string) error {
	fset := token.NewFileSet()

	ast, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}

	if len(ast.Comments) == 0 {
		return fmt.Errorf("%s: %w", path, errNoComments)
	}

	if ast.Comments[0].List[0].Text != goApache2LicenseHeader {
		return fmt.Errorf("%s: %w", path, errFirstCommentNotLicense)
	}

	return nil
}

// checkGoLicense finds all go source files in the working directory, then parses them
// into an AST and checks there is a license header in there.
func checkGoLicense() error {
	paths, err := glob(".go")
	if err != nil {
		return err
	}

	var hasErrors bool

	for _, path := range paths {
		if err := checkGoLicenseFile(path); err != nil {
			fmt.Println(err)

			hasErrors = true
		}
	}

	if hasErrors {
		return errFail
	}

	return nil
}

// main runs any license checkers over the code.
func main() {
	if err := checkGoLicense(); err != nil {
		os.Exit(1)
	}
}
