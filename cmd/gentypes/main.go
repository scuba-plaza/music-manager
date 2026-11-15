package main

import (
	"fmt"
	"os"

	"music-manager/internal/tsgen"
	"music-manager/internal/web"
)

func main() {
	out, err := tsgen.Generate(web.Contract()...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(out)
}
