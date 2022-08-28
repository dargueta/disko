/*
Package unixv1 implements the file system used by Unix version 1.

This implementation strictly adheres to the specifications linked to below, with
one crucial exception: timestamps are measured in seconds from the traditional
Unix epoch of 1970-01-01 00:00:00 UTC, not sixtieths of a second from
1971-01-01 00:00:00 in an undefined timezone. The reason why I do this is because
the definition in the manual gives a maximum representable date of 1973-04-08
12:06:28.25. (The traditional timestamp definition became the standard in 1973.)

Further implementation details gleaned from a draft version of the Unix V1
source code. Specifically, I looked at the boot code and hard-coded data as well
as the file system overview provided as an addendum to the man page in the
programmers' manual.

Draft source code for Unix v1: http://www.bitsavers.org/pdf/bellLabs/unix/PreliminaryUnixImplementationDocument_Jun72.pdf
Relevant parts are sections E.0 pp 4 & 7, F.6 (pages 9, 12, and 100+ in the PDF)

`man` page for the file system description: https://www.bell-labs.com/usr/dmr/www/man51.pdf
Relevant pages 8-11.

Resources can be found here: https://www.bell-labs.com/usr/dmr/www/1stEdman.html
*/

package unixv1
