package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Regexes
var (
	bibResourceRegex = regexp.MustCompile("\\\\addbibresource\\{(.+)\\}")
	usePackageRegex  = regexp.MustCompile("\\\\usepackage\\{(.+)\\}")
	bibTexRegex      = regexp.MustCompile("\\\\bibliography\\{(.+)\\}")
)

// Check a file for incremental builds
// bibBackend will either be empty (no bibliography will be generated), "biber" (preferred), or "bibtex" if required by the document
func checkFile(infile, outfile string) (skipBuild bool, bibBackend string) {
	bibDepends := getBibDepends(infile)
	if len(bibDepends) > 0 {
		bibBackend = getBibBackend(infile)
	}
	if pdfStat := stat(outfile); pdfStat != nil {
		if texStat := stat(infile); texStat != nil {
			// If the output PDF's modtime isn't older than the input tex (or any of its bibDepends), building it can be skipped
			// TODO: implement -force argument to disable incremental builds
			pdfModTime := (*pdfStat).ModTime().Unix()
			if (*texStat).ModTime().Unix() <= pdfModTime {
				/* Will be set to false if any bibDepends are newer than the output PDF
				or stat fails for any of them, disallowing the build from being skipped */
				skipBuild = true
				// Check biber dependency modtimes
				for _, bibPath := range bibDepends {
					bibStat := stat(bibPath)
					// Warn about missing bibDepends
					if bibStat == nil {
						pdfName := filepath.Base(outfile)
						bibName := filepath.Base(bibPath)
						log(fmt.Sprintf("Warning: missing bibDepend %s", bibName), pdfName)
						continue
					}
					if (*bibStat).ModTime().Unix() > pdfModTime {
						skipBuild = false
						break
					}
				}
			}
		}
	}
	return skipBuild, bibBackend
}

// Find all .bib files a .tex file depends on by searching for \addbibresource statements
func getBibDepends(path string) (bibDepends []string) {
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	// Open file for scanning
	scanner := bufio.NewScanner(file)
	dir := filepath.Dir(path)

	// Find \addbibresource or \bibliography lines
	for scanner.Scan() {
		line := scanner.Text()
		if matches := bibResourceRegex.FindStringSubmatch(line); len(matches) == 2 {
			bibDepends = append(bibDepends, filepath.Join(dir, matches[1]))
		} else if matches := bibTexRegex.FindStringSubmatch(line); len(matches) == 2 {
			bibDepends = append(bibDepends, filepath.Join(dir, matches[1]))
		}
	}
	return bibDepends
}

// Determine which TeXlive compiler backend to use for a file
func getBackend(path string) (backend string) {
	const (
		pdflatex = "pdflatex"
		lualatex = "lualatex"
	)

	// Open file for scanning
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(file)

	// Check if any packages requiring alternative backends are used
	for scanner.Scan() {
		line := scanner.Text()
		if matches := usePackageRegex.FindStringSubmatch(line); len(matches) == 2 {
			pkg := matches[1]
			switch pkg {
			case "unicode-math":
				return lualatex
			case "fontspec":
				return lualatex
			}
		}
	}
	// Default to pdflatex
	return pdflatex
}

// Determine which bibliography generation backend to use for a file
func getBibBackend(path string) (bibBackend string) {
	const (
		biber  = "biber"
		bibtex = "bibtex"
	)

	// Open file for scanning
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(file)

	// Check if any bibtex commands are used
	for scanner.Scan() {
		line := scanner.Text()
		if bibTexRegex.MatchString(line) {
			return bibtex
		}
	}

	// Default to biber
	return biber
}

var expectedOutputExts = map[string]bool{
	"aux": true,
	"log": true,
	"pdf": true}

// checkMultipass - Check if a file needs two compilation passes to be fully built
func checkMultipass(path string) bool {
	var err error
	path, err = filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	texBase := strings.TrimSuffix(path, ".tex")
	parts := strings.Split(path, "/")
	dir := strings.Join(parts[:len(parts)-1], "/")

	// Check which output files were produced
	files, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		parts := strings.SplitN(file.Name(), ".", 2)
		basename := parts[0]
		var ext string
		if len(parts) == 2 {
			ext = parts[1]
		}
		// If a file with a matching basename and unexpected extension is produced, perform a second compilation pass
		if basename == texBase || !expectedOutputExts[ext] {
			return true
		}
	}
	return false
}

// os.Stat wrapper designed for incremental builds:
// ignores os.IsNotExist errors, logs all other errors
func stat(path string) *fs.FileInfo {
	if stat, err := os.Stat(path); err == nil {
		return &stat
	} else if !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to run stat on %s - %s\n", path, err)
	}
	return nil
}

// Log output labeled with pdfName
func log(s, pdfName string) {
	fmt.Printf("\x1b[1m[%s]\x1b[0m: %s\n", pdfName, s)
}
