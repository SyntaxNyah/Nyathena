/* Athena - A server for Attorney Online 2 written in Go
Copyright (C) 2022 MangosArentLiterature <mango@transmenace.dev>

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>. */

// Package hamt implements a 32-ary Hash Array Mapped Trie (HAMT) keyed on
// uint32 values and a bespoke binary serialization format used by the Nyathena
// login challenge system.
//
// Wire format (big-endian throughout):
//
//	ArrayNode: tag(4) | bitmap(4) | children…
//	LeafNode:  tag(4) | key(4)    | value(4)
//
// The magic tag constants are intentionally opaque to make the format
// non-trivial to reverse-engineer without reading this source.
package hamt

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

// bitsPerLevel is the number of key bits consumed per trie level (giving a
// branching factor of 32).
const bitsPerLevel = 5

// levelMask masks the 5 least-significant bits of a shifted key to produce
// the child index at a given level.
const levelMask = (1 << bitsPerLevel) - 1

// Node type tags embedded in the binary wire format.
const (
	tagArray uint32 = 0x484D4149 // "HMAI"
	tagLeaf  uint32 = 0x484D4C46 // "HMLF"
)

// Node is the interface satisfied by both ArrayNode and LeafNode.
type Node interface{ isNode() }

// ArrayNode is an internal trie node.  Bitmap tracks which of the 32 child
// slots are occupied; Children is the corresponding sparse slice (one entry
// per set bit, in ascending bit-index order).
type ArrayNode struct {
	Bitmap   uint32
	Children []Node
}

// LeafNode stores a single key→value mapping.
type LeafNode struct {
	Key   uint32
	Value uint32
}

func (*ArrayNode) isNode() {}
func (*LeafNode) isNode()  {}

// HAMT is the root of the hash array mapped trie.
type HAMT struct {
	root *ArrayNode
}

// New returns an empty HAMT.
func New() *HAMT {
	return &HAMT{root: &ArrayNode{}}
}

// Root returns the root ArrayNode (for serialization).
func (h *HAMT) Root() *ArrayNode { return h.root }

// Insert stores key→value in the HAMT, replacing any existing value for key.
func (h *HAMT) Insert(key, value uint32) {
	h.root = insertArray(h.root, key, value, 0)
}

// Get returns the value associated with key and true, or 0 and false if the
// key is absent.
func (h *HAMT) Get(key uint32) (uint32, bool) {
	return getFromArray(h.root, key, 0)
}

// insertArray returns a (possibly new) ArrayNode with key→value inserted at
// the given shift level.
func insertArray(n *ArrayNode, key, value uint32, shift uint) *ArrayNode {
	idx := (key >> shift) & levelMask
	bitpos := uint32(1) << idx
	childIdx := bits.OnesCount32(n.Bitmap & (bitpos - 1))

	if n.Bitmap&bitpos == 0 {
		// Slot is empty: splice a new leaf in.
		newChildren := make([]Node, len(n.Children)+1)
		copy(newChildren, n.Children[:childIdx])
		newChildren[childIdx] = &LeafNode{Key: key, Value: value}
		copy(newChildren[childIdx+1:], n.Children[childIdx:])
		return &ArrayNode{Bitmap: n.Bitmap | bitpos, Children: newChildren}
	}

	// Slot is occupied: update the child in-place (copy-on-write).
	existing := n.Children[childIdx]
	var newChild Node
	switch e := existing.(type) {
	case *LeafNode:
		if e.Key == key {
			// Same key: update the value.
			newChild = &LeafNode{Key: key, Value: value}
		} else {
			// Collision: expand the existing leaf into a sub-array and insert
			// both entries one level deeper.
			sub := insertArray(&ArrayNode{}, e.Key, e.Value, shift+bitsPerLevel)
			sub = insertArray(sub, key, value, shift+bitsPerLevel)
			newChild = sub
		}
	case *ArrayNode:
		newChild = insertArray(e, key, value, shift+bitsPerLevel)
	default:
		// Should never happen.
		return n
	}

	newChildren := make([]Node, len(n.Children))
	copy(newChildren, n.Children)
	newChildren[childIdx] = newChild
	return &ArrayNode{Bitmap: n.Bitmap, Children: newChildren}
}

// getFromArray traverses the trie looking for key starting at shift.
func getFromArray(n *ArrayNode, key uint32, shift uint) (uint32, bool) {
	idx := (key >> shift) & levelMask
	bitpos := uint32(1) << idx
	if n.Bitmap&bitpos == 0 {
		return 0, false
	}
	childIdx := bits.OnesCount32(n.Bitmap & (bitpos - 1))
	switch child := n.Children[childIdx].(type) {
	case *LeafNode:
		if child.Key == key {
			return child.Value, true
		}
		return 0, false
	case *ArrayNode:
		return getFromArray(child, key, shift+bitsPerLevel)
	}
	return 0, false
}

// XORAllValues returns the XOR of every value stored in the subtree rooted at
// root.  This is the expected answer for the login challenge.
func XORAllValues(root *ArrayNode) uint32 {
	var acc uint32
	xorNode(root, &acc)
	return acc
}

func xorNode(n Node, acc *uint32) {
	switch node := n.(type) {
	case *ArrayNode:
		for _, child := range node.Children {
			xorNode(child, acc)
		}
	case *LeafNode:
		*acc ^= node.Value
	}
}

// Serialize encodes the HAMT into its binary wire format and returns the
// resulting byte slice.
func Serialize(root *ArrayNode) []byte {
	// Pre-allocate a reasonable buffer to reduce re-allocations.
	buf := make([]byte, 0, 256)
	serializeNode(&buf, root)
	return buf
}

func serializeNode(buf *[]byte, n Node) {
	switch node := n.(type) {
	case *ArrayNode:
		*buf = appendU32(*buf, tagArray)
		*buf = appendU32(*buf, node.Bitmap)
		for _, child := range node.Children {
			serializeNode(buf, child)
		}
	case *LeafNode:
		*buf = appendU32(*buf, tagLeaf)
		*buf = appendU32(*buf, node.Key)
		*buf = appendU32(*buf, node.Value)
	}
}

func appendU32(b []byte, v uint32) []byte {
	return append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// Deserialize parses a byte slice produced by Serialize and returns the root
// ArrayNode.  Returns an error if the data is malformed or truncated.
func Deserialize(data []byte) (*ArrayNode, error) {
	node, _, err := deserializeNode(data, 0)
	if err != nil {
		return nil, err
	}
	an, ok := node.(*ArrayNode)
	if !ok {
		return nil, fmt.Errorf("hamt: root node must be an ArrayNode")
	}
	return an, nil
}

func deserializeNode(data []byte, offset int) (Node, int, error) {
	if offset+4 > len(data) {
		return nil, offset, fmt.Errorf("hamt: unexpected EOF reading node tag at offset %d", offset)
	}
	tag := binary.BigEndian.Uint32(data[offset:])
	offset += 4

	switch tag {
	case tagArray:
		if offset+4 > len(data) {
			return nil, offset, fmt.Errorf("hamt: unexpected EOF reading bitmap at offset %d", offset)
		}
		bitmap := binary.BigEndian.Uint32(data[offset:])
		offset += 4
		count := bits.OnesCount32(bitmap)
		children := make([]Node, count)
		for i := 0; i < count; i++ {
			child, newOffset, err := deserializeNode(data, offset)
			if err != nil {
				return nil, newOffset, err
			}
			children[i] = child
			offset = newOffset
		}
		return &ArrayNode{Bitmap: bitmap, Children: children}, offset, nil

	case tagLeaf:
		if offset+8 > len(data) {
			return nil, offset, fmt.Errorf("hamt: unexpected EOF reading leaf at offset %d", offset)
		}
		key := binary.BigEndian.Uint32(data[offset:])
		value := binary.BigEndian.Uint32(data[offset+4:])
		return &LeafNode{Key: key, Value: value}, offset + 8, nil

	default:
		return nil, offset, fmt.Errorf("hamt: unknown node tag 0x%08x at offset %d", tag, offset-4)
	}
}
