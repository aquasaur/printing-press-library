package main

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/cli"
)

func main() {
	if err := cli.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(exitCodeFor(err))
	}
}

func exitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	if coded, ok := err.(cli.CodedError); ok {
		return coded.Code()
	}
	return 1
}
