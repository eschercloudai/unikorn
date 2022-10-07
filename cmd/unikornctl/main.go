package main

import (
	"os"

	"github.com/eschercloudai/unikorn/pkg/command"
)

func main() {
	c := command.Generate()

	if err := c.Execute(); err != nil {
		os.Exit(1)
	}
}
