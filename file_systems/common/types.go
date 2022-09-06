// Package common contains definitions of fundamental types and functions used
// across multiple file system implementations.
package common

import "math"

type LogicalBlock uint
type PhysicalBlock uint

const InvalidLogicalBlock = LogicalBlock(math.MaxUint)
const InvalidPhysicalBlock = PhysicalBlock(math.MaxUint)

// Truncator is an interface for objects that support a Truncate() method. This
// method must behave just like [os.File.Truncate].
type Truncator interface {
	Truncate(size int64) error
}
