package main

import (
	"fmt"
	"os"

	"github.com/musacanminaz/swelo/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "swelo:", err)
		os.Exit(1)
	}
}
