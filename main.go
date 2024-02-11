package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func main() {
	// Input/output files for single-file mode
	var infile, outfile string
	// Parse args
	for i, arg := range os.Args[:len(os.Args)-1] {
		if strings.HasPrefix(arg, "-o") {
			if len(arg) > 2 {
				outfile = arg[2:]
			} else if i < len(os.Args)-1 {
				outfile = os.Args[i+1]
			} else {
				fmt.Fprintln(os.Stderr, "Malformed outfile argument (-o)")
				os.Exit(1)
			}
		}
	}
	if in := os.Args[len(os.Args)-1]; strings.HasSuffix(in, ".tex") {
		infile = in
	}
	fmt.Println("Running TeXnotes compiler")
	var wg sync.WaitGroup

	// If an infile is provided, run in single-file mode
	if len(infile) > 0 {
		// Set default output file if -o is not provided
		if len(outfile) == 0 {
			outfile = strings.TrimSuffix(infile, ".tex") + ".pdf"
		}
		skipBuild, useBiber := checkFile(infile, outfile)
		if skipBuild {
			fmt.Printf("%s is up to date\n", outfile)
		} else {
			wg.Add(1)
			go buildPDF(infile, useBiber, &wg)
			wg.Wait()
		}
		return
	}

	// Build all TeX documents within cwd or subdirectories
	filepath.WalkDir(".", func(infile string, _ fs.DirEntry, _ error) error {
		// Skip non-tex files
		if !strings.HasSuffix(infile, ".tex") {
			return nil
		}

		// Check if built pdf exists and is up to date
		// (incremental builds)
		outfile := strings.TrimSuffix(infile, ".tex") + ".pdf"
		skipBuild, useBiber := checkFile(infile, outfile)
		if skipBuild {
			return nil
		}

		// Build PDF
		fmt.Println("Building", filepath.Base(outfile))
		wg.Add(1)
		go buildPDF(infile, useBiber, &wg)
		return nil
	})

	// Wait for all build processes to finish
	wg.Wait()
}
