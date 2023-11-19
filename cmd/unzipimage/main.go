package main

import (
	"fmt"
	"os"

	"github.com/dargueta/disko/utilities/compression"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(
			os.Stderr,
			"Uncompress a file using RLE8 and gzip.\nUsage: %s input-file output-file\n",
			os.Args[0])
		os.Exit(1)
	}

	sourceFilePath := os.Args[1]
	outputFilePath := os.Args[2]

	sourceFile, errSrc := os.Open(sourceFilePath)
	if errSrc != nil {
		fmt.Fprintf(
			os.Stderr, "Failed to open file for reading: `%v`: %s\n", sourceFilePath, errSrc)
		os.Exit(1)
	}
	defer sourceFile.Close()

	outFile, errOut := os.Create(outputFilePath)
	if errOut != nil {
		fmt.Fprintf(
			os.Stderr, "Failed to open file for writing: `%v`: %s\n", outputFilePath, errOut)
		os.Exit(1)
	}
	defer outFile.Close()

	nWritten, err := compression.DecompressImage(sourceFile, outFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error expanding file: %s\n", err)
		os.Exit(2)
	}

	fmt.Printf("Compressed input file to %d bytes.\n", nWritten)
}
