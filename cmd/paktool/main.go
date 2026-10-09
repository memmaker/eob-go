package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/felix/goeob/internal/pak"
)

func main() {
	fs, err := pak.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	for _, n := range fs.Names() {
		d, _ := fs.Read(n)
		fmt.Printf("%-16s %7d\n", n, len(d))
		if len(os.Args) > 2 {
			os.WriteFile(filepath.Join(os.Args[2], n), d, 0o644)
		}
	}
}
