// Package common contains definitions of fundamental types and functions used
// across multiple file system implementations.
package common

type LogicalBlock uint
type PhysicalBlock uint

// Truncator is an interface for objects that support a Truncate() method. This
// method must behave just like [os.File.Truncate].
type Truncator interface {
	Truncate(size int64) error
}
