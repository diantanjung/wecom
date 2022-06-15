package api

import (
	"bufio"
	"errors"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var templateString = `package %v
import "testing"

func Test_%v(t *testing.T) {
	  %v
}
`

func checkRemError(err error, filename string) error {
	if err != nil {
		os.Remove(filename)
		return err
	}
	return nil
}

func randString(length int) string {
	var ret string
	runes := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPKRSTUVWXYZ0123456789")
	for i := 0; i < length; i++ {
		ret += string(runes[rand.Intn(len(runes))])
	}
	return ret
}

func goExtractFuncDef(filepath, funcName string) ([]string, error) {
	regex := regexp.MustCompile(funcName + " *\\((.*)\\).*$")
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
			temp := strings.Split(strings.TrimSpace(v), " ")[0]
			// result[temp] = ""
			if temp != "" {
				result = append(result, temp)
			}
		}

	}
	return result, nil
}

func extractFilePackage(filepath string) (string, error) {
	regex, err := regexp.Compile("^ *package (.*)$")
	if err != nil {
		return "", err
	}
	fh, err := os.Open(filepath)
	f := bufio.NewReader(fh)

	if err != nil {
		return "", err
	}
	defer fh.Close()

	buf := make([]byte, 1024)
	for {
		buf, _, err = f.ReadLine()
		if err != nil {
			return "", errors.New("Package def in file " + filepath + " not found.")
		}
		s := string(buf)
		if regex.MatchString(s) {
			return strings.Split(string(buf), " ")[1], nil
		}
	}
}

func goExtractPackage(dirPath string) (string, string, error) {

	dirFiles, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return "", "", err
	}
	var goFiles []string
	for _, file := range dirFiles {
		if !file.IsDir() {
			isGoFile, _ := regexp.Match("\\.go$", []byte(file.Name()))
			if isGoFile {
				goFiles = append(goFiles, file.Name())
			}
		}
	}
	if len(goFiles) == 0 {
		err = errors.New("Err: No go source files were found.")
		return "", "", err
	}

	packageNames := make(map[string]string)
	for _, file := range goFiles {
		packageName, err := extractFilePackage(dirPath + file)
		if err != nil {
			return "", "", err
		}
		packageNames[packageName] = dirPath + file
	}
	if len(packageNames) > 1 {
		err = errors.New("Err: Multiple package definitions in the same directory")
		return "", "", err
	}

	var packageName, filePath string
	for k, v := range packageNames {
		packageName = k
		filePath = v
		break
	}
	return packageName, filePath, nil
}

func goGenerateAndFmtFile(testFileName, testFileDir, fileContent string) error {
	err := ioutil.WriteFile(testFileName, []byte(fileContent), 0644)
	if err != nil {
		return err
	}

	cmd := exec.Command("goimports", "-w=true", testFileName)
	cmd.Dir = testFileDir
	output, err := cmd.CombinedOutput()
	if len(output) == 0 && err != nil {
		err = checkRemError(err, testFileName)
		return err
	}
	return nil
}

func goRunFile(testname, filename, testFileDir string) (string, error) {
	cmd := exec.Command("/usr/local/go/bin/go", "test", "--run", testname)
	cmd.Dir = testFileDir
	output, err := cmd.CombinedOutput()
	stdout := string(output)
	stdoutLines := strings.Split(stdout, "\n")
	if len(output) == 0 && err != nil {
		err = checkRemError(err, filename)
		return "", err
	} else if err != nil {
		stdout2 := strings.Join(stdoutLines[:len(stdoutLines)-2], "\n")
		if len(stdout2) > 0 {
			err = errors.New(stdout2)
		} else {
			err = errors.New(stdout)
		}
		return stdout, err
	} else {
		stdout3 := strings.Join(stdoutLines[:len(stdoutLines)-3], "\n")
		if len(stdout3) > 0 {
			stdout = stdout3
		}
		return stdout, err
	}
}
