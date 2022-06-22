package api

import (
	"bufio"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var rsTemplateString = `mod %v;

fn main() {
    %v::%v;
}
`

func rsGetArgs(filepath, funcName string) ([]string, error) {
	regex := regexp.MustCompile("fn *" + funcName + " *\\((.*)\\).*$")
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

	splitArr := strings.Split(strings.TrimSpace(matchStr), ",")
	if len(splitArr) > 0 {
		for _, v := range splitArr {
			temp := strings.Split(strings.TrimSpace(v), ":")[0]
			// result[temp] = ""
			if temp != "" {
				result = append(result, temp)
			}
		}

	}
	return result, nil
}

func rsGenerateFile(testFileName, fileContent string) error {
	err := ioutil.WriteFile(testFileName, []byte(fileContent), 0644)
	if err != nil {
		return err
	}
	return nil
}

func rsRunFile(filename, modName, testFileDir string) (string, error) {
	cmd1 := exec.Command("rustc", filename)
	cmd1.Dir = testFileDir
	_, err := cmd1.CombinedOutput()

	cmd := exec.Command("./" + modName)
	cmd.Dir = testFileDir
	output, err := cmd.CombinedOutput()
	stdout := string(output)
	return stdout, err
}
