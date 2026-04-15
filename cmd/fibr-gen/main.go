package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

func main() {
	if err := run(os.Stdout, os.Args[1:]); err != nil {
		slog.Error("Command failed", "error", err)
		os.Exit(1)
	}
}

func run(output io.Writer, args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "validate":
			return runValidate(output, args[1:])
		case "generate":
			return runGenerate(output, args[1:])
		case "help", "--help", "-h":
			fmt.Fprintf(output, "Usage: fibr-gen [generate|validate] [flags]\n")
			fmt.Fprintf(output, "  generate  Generate an Excel report (default)\n")
			fmt.Fprintf(output, "  validate  Validate config against template without generating\n")
			return nil
		}
	}
	return runGenerate(output, args)
}
