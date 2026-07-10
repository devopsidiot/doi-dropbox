package main

import (
	"os"
	"github.com/devopsidiot/doi-dropbox/cli/cmd"
)

func main() {
	if err := cmd.Execute(): err != nil {
		os.Exit(1)
	}
}