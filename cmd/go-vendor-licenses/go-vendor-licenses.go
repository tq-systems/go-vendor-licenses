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
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"

	license "vgitlab01.tq-net.de/tq-em/tools/go-vendor-licenses.git/license"
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

func createManifest(manifest []metadata) error {
	writer := tabwriter.NewWriter(os.Stdout, 1, 4, 2, ' ', 0)

	for k := 0; k < len(manifest); k++ {
		pkgInfo := fmt.Sprintf("name:     %s\n", manifest[k].name)
		pkgInfo += fmt.Sprintf("revision: %s\n", manifest[k].revision)
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
		if err != nil {
			if ignoreCritLicsFlag {
				manifest[k].license = licenseString
			} else {
				fmt.Fprintln(os.Stderr, licenseString, err)
				os.Exit(1)
			}
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

	manifest := readGopkgFile()

	ignoreCritLicsFlag := flag.Bool("i", false,
		"ignore missing or copyleft licenses")
	manifestFlag := flag.Bool("m", false,
		"display manifest of dependant packages")
	disclaimerFlag := flag.Bool("d", false,
		"display disclaimer of dependant packages")
	flag.Parse()

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
		fmt.Print(`Usage: go-vendor-licenses [option] [mode]

option:
	-i		ignore crititcal licenses during creating the manifest

modes:
  -m    display dependencies manifest
  -d    display dependencies disclaimer

'go-vendor-licenses' is a go dependencies tool which can be used natively during developing or building.
The tool reads the 'Gopkg.lock' file and identifies the licenses in the vendor directory.
These are created by the go dependencies tool 'dep', so ensure running 'dep ensure' before using this tool.

`)
		os.Exit(1)
	}
}
