package api

import (
	"bufio"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var rktTemplateString = `#lang racket
(require "%v")
(%v)
`

func rktGetArgs(filepath, funcName string) ([]string, error) {
	regex := regexp.MustCompile("\\( *define *\\( *" + funcName + " *(.*) *\\)")
	fh, err := os.Open(filepath)
	f := bufio.NewReader(fh)

	if err != nil {
		return nil, err
	}
	result := make([]string, 0)
	defer fh.Close()

	buf := make([]byte, 1024)
	var matchStr = ""
	for {
		buf, _, err = f.ReadLine()
		if err != nil {
			break
		}
		s := string(buf)
		match := regex.FindStringSubmatch(s)
		if len(match) > 0 {
			matchStr = match[1]
		}
	}

	splitArr := strings.Split(strings.TrimSpace(matchStr), " ")
	if len(splitArr) > 0 {
		for _, v := range splitArr {
			temp := strings.Split(strings.TrimSpace(v), " ")[0]
			// result[temp] = ""
			if temp != "" {
				result = append(result, temp)
			}

		}

	}
	return result, nil
}

func rktGenerateFile(testFileName, fileContent string) error {
	err := ioutil.WriteFile(testFileName, []byte(fileContent), 0644)
	if err != nil {
		return err
	}
	return nil
}

func rktRunFile(filename, testFileDir string) (string, error) {
	cmd := exec.Command("/usr/racket/bin/racket", filename)
	cmd.Dir = testFileDir
	output, err := cmd.CombinedOutput()
	stdout := string(output)
	return stdout, err
}