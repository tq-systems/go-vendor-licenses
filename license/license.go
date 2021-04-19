/*
 * go-vendor-licenses - license.go
 * Copyright (c) 2018, TQ-Systems GmbH. All rights reserved.
 * Author: Christoph Krutz
 * Use of this source code is governed by a BSD-style license
 * that can be found in the LICENSE file.
 *
 * This work is based on github.com/pmezard/licenses
 */

package license

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/tq-systems/go-vendor-licenses/license/assets"
)

var (
	regexLicense = regexp.MustCompile(`(?i)^(?:` +
		`((?:un)?licen[sc]e)|` +
		`((?:un)?licen[sc]e\.(?:md|markdown|txt))|` +
		`(copy(?:ing|right)(?:\.[^.]+)?)|` +
		`(licen[sc]e\.[^.]+)` +
		`)$`)
	regexDisclaim = regexp.MustCompile(`(?i)^(?:` +
		`((?:un)?licen[sc]e[s]*)(?:\.[^.]+)?|` +
		`(copy[(?:ing|right)]*(?:\.[^.]+)?)|` +
		`(author[s]*)(?:\.[^.]+)?|` +
		`(contributor[s]*)(?:\.[^.]+)?|` +
		`(patent[s]*)(?:\.[^.]+)?` +
		`)$`)
	regexWords     = regexp.MustCompile(`[\w']+`)
	regexCopyright = regexp.MustCompile(
		`(?i)\s*Copyright (?:©|\(c\)|\xC2\xA9)?\s*(?:\d{4}|\[year\]).*`)
)

var (
	criticalLicenseNicknames = []string{
		"AGPL-3.0",
		"EPL-1.0",
		"GPL-2.0",
		"GPL-3.0",
		"MPL-2.0",
		"MS-RL",
		"NOLICENSE",
		"LGPL-2.1",
		"OSL-3.0",
	}
)

type Template struct {
	Title    string
	Nickname string
	Words    map[string]int
}

type License struct {
	Score        float64
	Template     *Template
	Path         string
	Err          string
	ExtraWords   []string
	MissingWords []string
}

type MatchResult struct {
	Template     *Template
	Score        float64
	ExtraWords   []string
	MissingWords []string
}

type Word struct {
	Text string
	Pos  int
}

type sortedWords []Word

func (s sortedWords) Len() int {
	return len(s)
}

func (s sortedWords) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sortedWords) Less(i, j int) bool {
	return s[i].Pos < s[j].Pos
}

func sortAndReturnWords(words []Word) []string {
	sort.Sort(sortedWords(words))
	tokens := []string{}
	for _, w := range words {
		tokens = append(tokens, w.Text)
	}
	return tokens
}

func matchTemplates(license []byte, templates []*Template) MatchResult {
	bestScore := float64(-1)
	var bestTemplate *Template
	bestExtra := []Word{}
	bestMissing := []Word{}
	words := makeWordSet(license)
	for _, t := range templates {
		extra := []Word{}
		missing := []Word{}
		common := 0
		for w, pos := range words {
			_, ok := t.Words[w]
			if ok {
				common++
			} else {
				extra = append(extra, Word{
					Text: w,
					Pos:  pos,
				})
			}
		}
		for w, pos := range t.Words {
			if _, ok := words[w]; !ok {
				missing = append(missing, Word{
					Text: w,
					Pos:  pos,
				})
			}
		}
		score := 2 * float64(common) / (float64(len(words)) + float64(len(t.Words)))
		if score > bestScore {
			bestScore = score
			bestTemplate = t
			bestMissing = missing
			bestExtra = extra
		}
	}
	return MatchResult{
		Template:     bestTemplate,
		Score:        bestScore,
		ExtraWords:   sortAndReturnWords(bestExtra),
		MissingWords: sortAndReturnWords(bestMissing),
	}
}

func cleanLicenseData(data []byte) []byte {
	data = bytes.ToLower(data)
	data = regexCopyright.ReplaceAll(data, nil)
	return data
}

func makeWordSet(data []byte) map[string]int {
	words := map[string]int{}
	data = cleanLicenseData(data)
	matches := regexWords.FindAll(data, -1)
	for i, m := range matches {
		s := string(m)
		if _, ok := words[s]; !ok {
			// Non-matching words are likely in the license header, to mention
			// copyrights and authors. Try to preserve the initial sequences,
			// to display them later.
			words[s] = i
		}
	}
	return words
}

func parseTemplate(content string) (*Template, error) {
	t := Template{}
	text := []byte{}
	state := 0
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if state == 0 {
			if line == "---" {
				state = 1
			}
		} else if state == 1 {
			if line == "---" {
				state = 2
			} else {
				if strings.HasPrefix(line, "title:") {
					t.Title = strings.TrimSpace(line[len("title:"):])
				} else if strings.HasPrefix(line, "nickname:") {
					t.Nickname = strings.TrimSpace(line[len("nickname:"):])
				}
			}
		} else if state == 2 {
			text = append(text, scanner.Bytes()...)
			text = append(text, []byte("\n")...)
		}
	}
	t.Words = makeWordSet(text)
	return &t, scanner.Err()
}

func loadTemplates() ([]*Template, error) {
	templates := []*Template{}
	for _, a := range assets.Assets {
		templ, err := parseTemplate(a.Content)
		if err != nil {
			return nil, err
		}
		templates = append(templates, templ)
	}
	return templates, nil
}

func matchDisclaimName(name string) bool {
	isDisclaimFile := regexDisclaim.MatchString(name)
	return isDisclaimFile
}

func scoreLicenseName(name string) float64 {
	m := regexLicense.FindStringSubmatch(name)
	switch {
	case m == nil:
		break
	case m[1] != "":
		return 1.0
	case m[2] != "":
		return 0.9
	case m[3] != "":
		return 0.8
	case m[4] != "":
		return 0.7
	}
	return 0.0
}

func findLicenseFile(path string) (string, error) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		return "", err
	}

	bestScore := float64(0)
	bestName := ""
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		score := scoreLicenseName(file.Name())
		if score > bestScore {
			bestScore = score
			bestName = file.Name()
		}
	}
	if bestName != "" {
		return filepath.Join(path, bestName), nil
	}
	return "", nil
}

func identifyLicense(path string) (*License, error) {
	templates, err := loadTemplates()
	if err != nil {
		return nil, err
	}

	licenseFile, err := findLicenseFile(path)
	license := License{
		Path: licenseFile,
	}

	data, err := ioutil.ReadFile(licenseFile)
	if err != nil {
		return nil, err
	}

	match := matchTemplates(data, templates)

	license.Score = match.Score
	license.Template = match.Template
	license.ExtraWords = match.ExtraWords
	license.MissingWords = match.MissingWords

	return &license, nil
}

func BuildLicenseString(path string) (string, error) {
	confidence := 0.95

	license, err := identifyLicense(path)
	if err != nil {
		return "", fmt.Errorf("Unable to identify license of %s: %s", path, err.Error())
	}

	for _, match := range criticalLicenseNicknames {
		if match == license.Template.Nickname {
			log.Println("Found critical license: ", license.Template.Nickname)
			err = fmt.Errorf("criticalLicense")
		}
	}

	licenseString := "?"
	if license.Template != nil {
		if license.Score >= confidence {
			licenseString = fmt.Sprintf("%s (%2d%%)",
				license.Template.Title, int(100*license.Score))
		} else {
			licenseString = fmt.Sprintf("%s (%2d%%)",
				license.Template.Title, int(100*license.Score))
			if len(license.ExtraWords) > 0 {
				licenseString += "\n\t+words: " + strings.Join(license.ExtraWords, ", ")
			}
			if len(license.MissingWords) > 0 {
				licenseString += "\n\t-words: " + strings.Join(license.MissingWords, ", ")
			}
		}
	} else if license.Err != "" {
		licenseString = strings.Replace(license.Err, "\n", " ", -1)
	}
	return licenseString, err
}

func BuildDisclaimerString(path string, pkg string) error {
	var disclaimer string = ""
	writer := tabwriter.NewWriter(os.Stdout, 1, 4, 2, ' ', 0)

	disclaimer += fmt.Sprintf("\nDISCLAIMER of %s:\n", pkg)

	files, err := ioutil.ReadDir(filepath.Join(path))
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.Mode().IsRegular() {
			continue
		}

		if matchDisclaimName(file.Name()) {
			disclaimer += fmt.Sprintf("\nFilename: %s\n", filepath.Base(file.Name()))
			content, err := ioutil.ReadFile(filepath.Join(path, file.Name()))
			if err != nil {
				log.Println(err)
			}
			disclaimer += fmt.Sprintf("%s", content)
		}
	}

	_, err2 := writer.Write([]byte(disclaimer + "\n"))
	if err2 != nil {
		return err2
	}

	writer.Flush()
	return nil
}
