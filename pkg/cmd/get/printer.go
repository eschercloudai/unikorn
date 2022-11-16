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

package get

import (
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
)

// printResult is a common printing function that accepts tabulated GET data
// from the Kubernetes API.
func (o *getPrintFlags) printResult(r *resource.Result) error {
	infos, err := r.Infos()
	if err != nil {
		return err
	}

	// Assume we have a single object, the r.Err above will crap out if no results are
	// found.  We know all returned results will be projects.  If doing a human printable
	// get, then a single table will be returned.  If getting by name, especially multiple
	// names, then there may be multiple results.  Coalesce these into a single list
	// as that's what is expected from standard tools.
	object := infos[0].Object

	if len(infos) > 1 {
		list := &corev1.List{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "List",
			},
		}

		for _, info := range infos {
			list.Items = append(list.Items, runtime.RawExtension{Object: info.Object})
		}

		object = list
	}

	printer, err := o.toPrinter()
	if err != nil {
		return err
	}

	if err := printer.PrintObj(object, os.Stdout); err != nil {
		return err
	}

	return nil
}
