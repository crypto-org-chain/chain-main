// Copyright (c) 2014 Will Fitzgerald. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
package types

import (
	"errors"
	"math/bits"
)

// the wordSize of a bit set
const wordSize = uint(64)

var errorOutOfBound = errors.New("out of bound")

// A BitSet is a set of bits.
// Subscription module only needs 60 bits at most, a single uint64 is more than enough.
type BitSet struct {
	set uint64
}

func NewBitSet() BitSet {
	return BitSet{}
}

// Set bit i to 1
// if i >= 64, return an error
func (b *BitSet) Set(i uint) error {
	if i >= wordSize {
		return errorOutOfBound
	}
	b.set |= 1 << i
	return nil
}

// Clear bit i to 0
// if i >= 64, return an error
func (b *BitSet) Clear(i uint) error {
	if i >= wordSize {
		return errorOutOfBound
	}
	b.set &^= 1 << i
	return nil
}

// Test whether bit i is set.
// if i >= 64, return false
func (b *BitSet) Test(i uint) bool {
	if i >= wordSize {
		return false
	}
	return b.set&(1<<i) != 0
}

// NextSet returns the next bit set from the specified index,
// including possibly the current index
// along with an error code (true = valid, false = no set bit found)
// for i,e := v.NextSet(0); e; i,e = v.NextSet(i + 1) {...}
//
// Users concerned with performance may want to use NextSetMany to
// retrieve several values at once.
func (b *BitSet) NextSet(i uint) (uint, bool) {
	if i >= wordSize {
		return 0, false
	}
	w := b.set >> i
	if w != 0 {
		return i + uint(bits.TrailingZeros64(w)), true
	}
	return 0, false
}

// func (b BitSet) Union(other BitSet) BitSet {
// 	return BitSet{set: b.set | other.set}
// }

func (b *BitSet) InPlaceUnion(other BitSet) {
	b.set |= other.set
}

func (b BitSet) Intersection(other BitSet) BitSet {
	return BitSet{set: b.set & other.set}
}

// func (b *BitSet) InPlaceIntersection(other BitSet) {
// 	b.set &= other.set
// }

func (b BitSet) Len() uint {
	return uint(bits.OnesCount64(b.set))
}
