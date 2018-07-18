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
	"os"
	"strings"
	"text/tabwriter"

	license "tq-git-pr1.tq-net.de/tq-em/base/go-vendor-licenses.git/license"
)

const version string = "0.1"

type metadata struct {
	name     string
	revision string
	version  string
	branch   string
	license  string
}

var (
	manifest  map[int]*metadata
	gopkgFile string = "Gopkg.lock"
	vendorDir string = "vendor"
)

func readGopkgFile() {
	var i int = 0

	file, error := os.Open(gopkgFile)
	if error != nil {
		fmt.Println(error)
	}
	defer file.Close()

	meta := metadata{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		if line == "[[projects]]" || line == "[solve-meta]" {

			if meta.name != "" {
				//fmt.Println(meta)
				manifest[i] = &metadata{
					meta.name,
					meta.revision,
					meta.version,
					meta.branch,
					meta.license}
				i++
			}

			meta = metadata{"", "", "", "", ""}
			continue
		}

		if strings.Contains(line, "=") {
			split := strings.Split(scanner.Text(), "=")
			key := strings.Trim(split[0], " ")
			value := strings.Trim(split[1], "\" []")

			switch key {
			case "name":
				meta.name = value
			case "revision":
				meta.revision = value
			case "version":
				meta.version = value
			case "branch":
				meta.branch = value
			}
		}
	}
	return
}

func createManifest() error {
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

func identifyLicenses() {
	for k := 0; k < len(manifest); k++ {
		licenseString, err := license.BuildLicenseString(manifest[k].name)
		if err != nil {
			fmt.Println(licenseString, err)
			os.Exit(1)
		} else {
			manifest[k].license = licenseString
		}
	}
}

func createDisclaimer() error {
	for k := 0; k < len(manifest); k++ {
		err := license.BuildDisclaimerString(manifest[k].name)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	manifest = make(map[int]*metadata)
	readGopkgFile()

	manifestFlag := flag.Bool("m", false,
		"display manifest of dependant packages")
	disclaimerFlag := flag.Bool("d", false,
		"display disclaimer of dependant packages")
	flag.Parse()

	if *manifestFlag {
		identifyLicenses()
		err := createManifest()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else if *disclaimerFlag {
		err := createDisclaimer()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		fmt.Printf(`Usage: vendor-licenses [mode]

modes:
  -m    display dependencies manifest
  -d    display dependencies disclaimer

'vendor-licenses' reads the 'Gopkg.lock' file and analyzes the vendor directory which are created by the go dependency tool 'dep'.
Ensure running 'dep ensure' before using this tool.

`)
		os.Exit(1)
	}
}
