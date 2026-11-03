package main

import (
	"fmt"
	"os"

	"ddbot/bot/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
