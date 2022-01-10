/*
 * go-vendor-licenses - go-vendor-licenses.go
 * Copyright (c) 2018, TQ-Systems GmbH. All rights reserved.
 * Author: Christoph Krutz
 * Use of this source code is governed by a BSD-style license
 * that can be found in the LICENSE file.
 */

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"

	license "github.com/tq-systems/go-vendor-licenses/license"
)

const version string = "0.2"

type metadata struct {
	name     string
	path     string
	revision string
	version  string
	branch   string
	license  string
}

const (
	gopkgFile = "Gopkg.lock"
)

var (
	ignoreCritLicsFlag = flag.Bool("i", false, "ignore missing or copyleft licenses")
	vendorFlag         = flag.Bool("vendor", false, "use vendored versions of dependant Go modules")
	manifestFlag       = flag.Bool("m", false, "display manifest of dependant packages")
	disclaimerFlag     = flag.Bool("d", false, "display disclaimer of dependant packages")
)

func buildPath(pkgname string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	path := filepath.Join(cwd, "vendor", pkgname)
	return path
}

func readGopkgFile() []metadata {
	ret := []metadata{}

	file, error := os.Open(gopkgFile)
	if error != nil {
		log.Fatalln(error)
	}
	defer file.Close()

	meta := metadata{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if line == "[[projects]]" || line == "[solve-meta]" {

			if meta.name != "" {
				ret = append(ret, meta)
			}

			meta = metadata{}
			continue
		}

		if strings.Contains(line, "=") {
			split := strings.Split(scanner.Text(), "=")
			key := strings.Trim(split[0], " ")
			value := strings.Trim(split[1], "\" []")

			switch key {
			case "name":
				meta.name = value
				meta.path = buildPath(value)
			case "revision":
				meta.revision = value
			case "version":
				meta.version = value
			case "branch":
				meta.branch = value
			}
		}
	}

	return ret
}

func modCommand(cmd string) *exec.Cmd {
	return exec.Command("sh", "-c", "GO111MODULE=on exec "+cmd)
}

func readModule() []metadata {
	var cmd *exec.Cmd
	if *vendorFlag {
		cmd = modCommand("go list -m -json -mod=mod all")
	} else {
		/* If we aren't using vendored dependencies, we need to make
		 * sure that all dependencies are available */
		err := modCommand("go mod download").Run()
		if err != nil {
			log.Fatalln(err)
		}
		cmd = modCommand("go list -m -json all")
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalln(err)
	}

	err = cmd.Start()
	if err != nil {
		log.Fatalln(err)
	}

	dec := json.NewDecoder(output)

	// There are more values, but we are only interested in these
	type module struct {
		Path    string
		Version string
		Dir     string
	}

	ret := []metadata{}

	for {
		var m module
		err := dec.Decode(&m)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalln(err)
		}

		meta := metadata{
			name:    m.Path,
			version: m.Version,
			path:    m.Dir,
		}

		ret = append(ret, meta)
	}

	err = cmd.Wait()
	if err != nil {
		log.Fatalln(err)
	}

	return ret
}

func createManifest(manifest []metadata) error {
	writer := tabwriter.NewWriter(os.Stdout, 1, 4, 2, ' ', 0)

	for k := 0; k < len(manifest); k++ {
		pkgInfo := fmt.Sprintf("name:     %s\n", manifest[k].name)
		if manifest[k].revision != "" {
			pkgInfo += fmt.Sprintf("revision: %s\n", manifest[k].revision)
		}
		if manifest[k].version != "" {
			pkgInfo += fmt.Sprintf("version:  %s\n", manifest[k].version)
		}
		if manifest[k].branch != "" {
			pkgInfo += fmt.Sprintf("branch:   %s\n", manifest[k].branch)
		}
		pkgInfo += fmt.Sprintf("license:  %s\n", manifest[k].license)

		_, err := writer.Write([]byte(pkgInfo + "\n"))
		if err != nil {
			return err
		}
	}
	writer.Flush()
	return nil
}

func identifyLicenses(manifest []metadata, ignoreCritLicsFlag bool) {
	for k := 0; k < len(manifest); k++ {
		licenseString, err := license.BuildLicenseString(manifest[k].path)
		if err != nil && !ignoreCritLicsFlag {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		} else {
			manifest[k].license = licenseString
		}
	}
}

func createDisclaimer(manifest []metadata) error {
	for k := 0; k < len(manifest); k++ {
		err := license.BuildDisclaimerString(manifest[k].path, manifest[k].name)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if runtime.GOOS != "linux" {
		log.Fatalf("Error: This tool is running in linux only!")
	}

	flag.Parse()
	if *manifestFlag == *disclaimerFlag {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	_, err := os.Stat(gopkgFile)
	if err != nil && !os.IsNotExist(err) {
		log.Fatalln(err)
	}

	var manifest []metadata

	if err == nil {
		manifest = readGopkgFile()
	} else {
		manifest = readModule()
	}

	if *manifestFlag {
		identifyLicenses(manifest, *ignoreCritLicsFlag)
		err := createManifest(manifest)
		if err != nil {
			log.Fatalln(err)
		}
	} else if *disclaimerFlag {
		err := createDisclaimer(manifest)
		if err != nil {
			log.Fatalln(err)
		}
	} else {
		// Should not be reached
		panic("invalid flag combination")
	}
}
