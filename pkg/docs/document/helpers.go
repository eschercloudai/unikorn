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
	"errors"
	"fmt"
	"sort"
)

var (
	ErrGroup = errors.New("group error")
)

func (l ParameterList) Len() int {
	return len(l)
}

func (l ParameterList) Less(i, j int) bool {
	return l[i].Name < l[j].Name
}

func (l ParameterList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l ResponseList) Len() int {
	return len(l)
}

func (l ResponseList) Less(i, j int) bool {
	return l[i].Status < l[j].Status
}

func (l ResponseList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l PathList) Len() int {
	return len(l)
}

func (l PathList) Less(i, j int) bool {
	return l[i].Path < l[j].Path
}

func (l PathList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l OperationList) Len() int {
	return len(l)
}

func (l OperationList) Less(i, j int) bool {
	return l[i].Method < l[j].Method
}

func (l OperationList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l GroupList) AddPath(p *Path) error {
	for _, group := range l {
		if group.ID == p.GroupID {
			group.Paths = append(group.Paths, p)

			sort.Stable(group.Paths)

			return nil
		}
	}

	return fmt.Errorf("%w: unable to locate group ID %s", ErrGroup, p.GroupID)
}
