package main

import (
	"fmt"
	"os"

	"github.com/MagnumGoYB/aitok/internal/cli"
)

func main() {
	cmd := cli.New(cli.App{Out: os.Stdout, Err: os.Stderr})
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
