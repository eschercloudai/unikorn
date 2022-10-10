package main

import (
	"os"

	"github.com/eschercloudai/unikorn/pkg/cmd"
)

func main() {
	c := cmd.Generate()

	if err := c.Execute(); err != nil {
		os.Exit(1)
	}
}
