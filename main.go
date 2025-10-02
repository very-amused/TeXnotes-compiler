package main

import (
	"errors"
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
	// Whether to delete all pdf files associated with tex files
	var clean bool
	// Parse args
	for i, arg := range os.Args[:len(os.Args)-1] {
		if arg == "clean" {
			clean = true
			fmt.Fprintln(os.Stderr, "'clean' argument was passed")
			continue
		}
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

	// Clean tex-associated PDF files if requested
	if clean {
		filepath.WalkDir(".", func(infile string, _ fs.DirEntry, _ error) error {
			// Skip non-tex files
			if !strings.HasSuffix(infile, ".tex") {
				return nil
			}

			// Delete PDF
			outfile := strings.TrimSuffix(infile, ".tex") + ".pdf"
			err := os.Remove(outfile)
			if err == nil {
				log("Deleted", outfile)
			} else if !errors.Is(err, os.ErrNotExist) {
				log(fmt.Sprintf("Failed to delete: %s", err), outfile)
			}

			return nil
		})
		return
	}

	// If an infile is provided, run in single-file mode
	if len(infile) > 0 {
		// Set default output file if -o is not provided
		if len(outfile) == 0 {
			outfile = strings.TrimSuffix(infile, ".tex") + ".pdf"
		}
		skipBuild, bibBackend := checkFile(infile, outfile)
		if skipBuild {
			fmt.Printf("%s is up to date\n", outfile)
		} else {
			wg.Add(1)
			go buildPDF(infile, bibBackend, &wg)
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
