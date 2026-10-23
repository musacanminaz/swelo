package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/musacanminaz/swelo/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		if errors.Is(err, cli.ErrCancelled) {
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "swelo:", err)
		os.Exit(1)
	}
}
