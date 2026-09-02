package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/arrowwhi/kinolist/internal/app"
)

func main() {
	envFile := flag.String("env", ".env", "path to the env file")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(app.Version())
		return
	}

	if err := app.Run(context.Background(), *envFile); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}
