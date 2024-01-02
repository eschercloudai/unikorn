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

package trie

import (
	"bufio"
	"io"
)

// trieMap maps a character to a new trieNode.
type trieMap map[rune]*trieNode

// trieNode contains possible descendents and whether this consitutes the end of a word.
type trieNode struct {
	// word indicates that this character can terminate a word.
	word bool

	// children is a map of possible next characters.
	children trieMap
}

// Trie defines an N-ary tree where a dictionary is loaded character by
// character into a new descendant.
type Trie struct {
	root *trieNode
}

// New creates a new dictionary trie.
func New() *Trie {
	return &Trie{
		root: &trieNode{
			children: trieMap{},
		},
	}
}

// AddWord adds a word to the trie.
func (t *Trie) AddWord(word string) {
	root := t.root

	for _, c := range word {
		if _, ok := root.children[c]; !ok {
			root.children[c] = &trieNode{
				children: trieMap{},
			}
		}

		root = root.children[c]
	}

	root.word = true
}

// AddDictionary adds a dictionary to the trie.
func (t *Trie) AddDictionary(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanWords)

	for scanner.Scan() {
		t.AddWord(scanner.Text())
	}

	return scanner.Err()
}

// CheckWord sees if the word exists in the trie.
func (t *Trie) CheckWord(word string) bool {
	root := t.root

	for _, c := range word {
		if _, ok := root.children[c]; !ok {
			return false
		}

		root = root.children[c]
	}

	return root.word
}
