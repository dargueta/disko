// Package compression provides several tools to compress file system images.
//
// File systems are broken up into fixed-size sectors, usually of 128 or 512
// bytes each. The emptier an image is, the more sectors consisting of entirely
// null bytes there will be. This means that "large" disk images (32 MiB) are
// mostly dead space we don't actually need to store.
//
// To reduce the size of the repository, we want to compress our testing disk
// images as much as we can. In experiments, the best compression was achieved
// by run-length encoding the raw image first, then using gzip on the result.
// An IBM 8" image of 256,256 bytes can be compressed to 3,009 bytes with only
// run-length encoding (98.8%). Compressing this with gzip results in a final
// size of 67 bytes -- a compression ratio of 99.97%.
//
// There are a variety of run-length encodings; this document refers strictly to
// the algorithm used by the Microsoft BMP file format, also known as RLE8. A
// brief explanation: if a byte B occurs N times where N >= 2, B is written twice,
// followed by a third (unsigned) byte indicating how many additional times B
// occurred. For example:
//
// 		WXXXXXXXXXXXXXXXYZZ
//		W XX 13 Y ZZ 0
//
// This scheme lets us represent runs of up to 257 bytes with three bytes. For
// runs longer than 257 bytes, they are treated as separate runs. For example,
// a run of 300 "X" is represented as `XX 255 XX 41`. Unfortunately, using a byte
// as its own escape sequence means that occurrences of the same byte exactly
// twice are stored as three bytes: the two bytes followed by a null byte
// indicating no further repetition.

package compression
