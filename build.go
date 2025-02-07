package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// buildPDF - Build a PDF using latex, optionally applying the full latex -> biber -> latex x2 pipeline for bibliography,
// intended to run on its own goroutine for optimal parallelization
func buildPDF(texPath string, useBiber bool, wg *sync.WaitGroup) {
	outDir := filepath.Dir(texPath)
	relPath, err := filepath.Rel(outDir, texPath)
	if err != nil {
		panic("Failed to get relative texPath")
	}
	// Determine backend
	backend := getBackend(texPath)
	// Configure logging
	pdfName := filepath.Base(strings.TrimSuffix(texPath, ".tex") + ".pdf")
	log := func(s string) {
		fmt.Printf("\x1b[1m[%s]\x1b[0m: %s\n", pdfName, s)
	}

	latex := func() {
		cmd := exec.Command(backend,
			"-halt-on-error", // halt-on-error is critical to prevent any latex jobs from entering an interactive prompt during build
			relPath)
		cmd.Dir = outDir
		cmd.Stderr = cmd.Stdout
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			panic("Failed to open pipe for backend command stdout")
		}
		scanner := bufio.NewScanner(stdout)
		cmd.Start()
		for scanner.Scan() {
			log(scanner.Text())
		}
		// Wait for process to close
		cmd.Wait()
	}
	biber := func() {
		bcfPath := strings.Replace(relPath, ".tex", ".bcf", 1)
		cmd := exec.Command("biber",
			bcfPath)
		cmd.Dir = outDir
		cmd.Stderr = cmd.Stdout
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			panic("Failed to open pipe for backend command stdout")
		}
		scanner := bufio.NewScanner(stdout)
		cmd.Start()
		for scanner.Scan() {
			log(scanner.Text())
		}
		// Wait for process to close
		cmd.Wait()
	}

	latex()
	if useBiber {
		// Run latex + biber + latex to generate bibliography
		latex()
		biber()
		latex()
	} else if checkMultipass(texPath) {
		latex()
	}

	log("Done!")
	wg.Done()
}
