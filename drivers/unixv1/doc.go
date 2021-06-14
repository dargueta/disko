/*
Package unixv1 implements the file system used by Unix version 1.

The original documentation can be found here: http://man.cat-v.org/unix-1st/5/file

This implementation strictly adheres to the specification, with the single
exception that timestamps are measured from the traditional Unix epoch of
1970-01-01, not 1971-01-01 as stated in the documentation. I'm 99.9% sure that's
a typo.
*/

package unixv1
