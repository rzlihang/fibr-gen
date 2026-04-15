package main

import (
	"fmt"
	"io"
)

func runValidate(output io.Writer, args []string) error {
	fmt.Fprintln(output, "validate: not yet implemented")
	return nil
}
