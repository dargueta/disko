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
//
// For truly "large" images -- e.g. an early 90s hard drive around 320MiB -- we
// could have a run of a megabyte of empty space. Simple RLE8 will compress this
// to 12,243 bytes. If instead of limiting ourselves to one byte for the run
// length we encode the length with ULEB128, we can get this down to just 5 bytes.
// That advantage is nearly eliminated though once we gzip the result; the RLE8
// shrinks to 50 bytes, ULEB128 expands from 5 to 25 bytes. In order to reduce
// the complexity of the code, we will (for now) stick to RLE8.

package compression
