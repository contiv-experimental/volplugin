package main

import (
	"fmt"
	"os"

	"github.com/codegangsta/cli"
)

func errExit(ctx *cli.Context, err error) {
	fmt.Printf("\nError: %v\n\n", err)
	cli.ShowAppHelp(ctx)
	os.Exit(1)
}
