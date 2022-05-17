package main

import (
	"log"
	"os"

	"github.com/urfave/cli/v2"
)

func main() {
	cli := cli.App{
		Usage: "Manage various types of disk image files",
		Commands: []*cli.Command{
			{
				Name:      "format",
				Usage:     "Create or wipe an image",
				Action:    formatImage,
				ArgsUsage: "HCL_FILE  KML_FILE",
			},
		},
	}

	err := cli.Run(os.Args)
	if err != nil {
		log.Fatalf("fatal error: %s", err.Error())
	}
}

func formatImage(context *cli.Context) error {
	return nil
}
