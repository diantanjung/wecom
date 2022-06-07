package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	db "github.com/diantanjung/wecom/db/sqlc"
	"github.com/diantanjung/wecom/token"
	"github.com/gin-gonic/gin"
)

type commandResponse struct {
	Path    string `json:"path"`
	Command string `json:"command"`
	Message string `json:"message"`
}

type getFileContentResponse struct {
	FileStr string `json:"file_str"`
}

type getFileContentRequest struct {
	PathStr string `json:"path_str" binding:"required"`
}

func (server *Server) GetFileContent(ctx *gin.Context) {
	var req getFileContentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	file := req.PathStr

	// file
	filePath := "/" + file
	fileString, err := ioutil.ReadFile(filePath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	res := getFileContentResponse{
		FileStr: strings.Trim(string(fileString), " "),
	}
	ctx.JSON(http.StatusOK, res)
}

type updateFileContentRequest struct {
	PathStr string `json:"path_str" binding:"required"`
	FileStr string `json:"file_str" binding:"required"`
}

func (server *Server) UpdateFileContent(ctx *gin.Context) {
	//if !server.isUserHasDir(ctx) {
	//	err := errors.New("Directory doesn't belong to the authenticated user")
	//	ctx.JSON(http.StatusUnauthorized, errorResponse(err))
	//	return
	//}
	var req updateFileContentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	file := req.PathStr

	pathFile := "/" + file

	err := ioutil.WriteFile(pathFile, []byte(req.FileStr), 0644)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	res := commandResponse{
		Message: "Success update file",
	}

	ctx.JSON(http.StatusOK, res)
}

func (server *Server) isUserHasDir(ctx *gin.Context) (res bool) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	req := db.CheckUserDirParams{
		UserID: authPayload.UserID,
		Name:   ctx.Param("dir"),
	}
	_, err := server.querier.CheckUserDir(ctx, req)
	if err == nil {
		return true
	}
	return false
}

func (server *Server) isUserHasDirectory(ctx *gin.Context, userId int64, dirName string) (res bool) {
	req := db.CheckUserDirParams{
		UserID: userId,
		Name:   dirName,
	}
	_, err := server.querier.CheckUserDir(ctx, req)
	if err == nil {
		return true
	}
	return false
}

type dirContent struct {
	Id       int    `json:"id"`
	Filename string `json:"filename"`
	IsDir    bool   `json:"isdir"`
	Size     int64  `json:"size"`
	Path     string `json:"path"`
	ModTime  string `json:"mod_time"`
}

type runCommandRequest struct {
	PathStr  string `json:"path_str" binding:"required"`
	Username string `json:"username" binding:"required"`
}

func (server *Server) RunCommand(ctx *gin.Context) {
	var req runCommandRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// file
	fullPath := "/" + req.PathStr
	runnerArr := strings.Split(fullPath, "/")
	runnerDir := strings.Join(runnerArr[:(len(runnerArr)-1)], "/")
	if fileInfo, err := os.Stat(fullPath); err != nil || fileInfo.IsDir() {
		err = errors.New("Command or file not found.")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	exeCmd := exec.Command(fullPath)
	exeCmd.Dir = runnerDir
	var out bytes.Buffer
	var stderr bytes.Buffer
	exeCmd.Stdout = &out
	exeCmd.Stderr = &stderr
	err := exeCmd.Run()
	var message string
	if err != nil {
		message = stderr.String()
	} else {
		if len(out.String()) > 0 {
			message = out.String()
		} else {
			message = "Succes to execute command."
		}
	}

	res := commandResponse{
		Path:    req.PathStr,
		Message: message,
	}

	ctx.JSON(http.StatusOK, res)
}

type runFuncRequest struct {
	PathStr  string `form:"path_str" binding:"required"`
	Username string `form:"username" binding:"required"`
	Args     string `form:"args" binding:"required"`
}

func (server *Server) RunFunc(ctx *gin.Context) {
	var req runFuncRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	args:=make(map[string]string)
	json.Unmarshal([]byte(req.Args), &args)

	argsStr := ""
	
	for _, value := range args {
		if _, err := strconv.Atoi(value); err != nil {
			argsStr += "\"" +value + "\","
		}else{
			argsStr += value + ","
		}
    }

	if last := len(argsStr) - 1; last >= 0 && argsStr[last] == ',' {
        argsStr = argsStr[:last]
    }

	fileArr := strings.Split(req.PathStr, "/")
	fileDir := "/" + strings.Join(fileArr[:(len(fileArr)-1)], "/") + "/"

	packageName, err := extractPackage(fileDir)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	functionCall := fileArr[(len(fileArr)-1)] + "("+ argsStr +")"

	testRandomName := randString(10)
	testFileName := fileDir + testRandomName + "_test.go"
	fileContent := fmt.Sprintf(templateString, packageName, testRandomName, functionCall)
	err = generateAndFmtFile(testFileName, fileDir, fileContent)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	defer os.Remove(testFileName)

	msg, err := runFile(testRandomName, testFileName, fileDir)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	res := commandResponse{
		Path: 		functionCall,
		Message: 	msg,
	}

	ctx.JSON(http.StatusOK, res)
}

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

func extractPackage(dirPath string) (string, error) {
	dirFiles, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return "", err
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
		if err != nil {
			return "", err
		}
	}
	packageNames := make(map[string]bool)
	for _, file := range goFiles {
		packageName, err := extractFilePackage(file)
		if err != nil {
			return "", err
		}
		packageNames[packageName] = true
	}
	if len(packageNames) > 1 {
		log.Fatal("Err: Multiple package definitions in the same directory")
	}

	var packageName string
	for k := range packageNames {
		packageName = k
		break
	}
	return packageName, nil
}

func generateAndFmtFile(testFileName, testFileDir, fileContent string) error {
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

func runFile(testname, filename, testFileDir string) (string, error) {
	cmd := exec.Command("go", "test", "--run", testname)
	cmd.Dir = testFileDir
	output, err := cmd.CombinedOutput()
	stdout := string(output)
	stdoutLines := strings.Split(stdout, "\n")
	if len(output) == 0 && err != nil {
		err = checkRemError(err, filename)
		fmt.Println("11 : " + stdout)
		return "", err
	} else if err != nil {
		stdout = strings.Join(stdoutLines[:len(stdoutLines)-2], "\n")
		err = errors.New(stdout)
		return stdout, err
	} else {
		stdout = strings.Join(stdoutLines[:len(stdoutLines)-3], "\n")
		return stdout, err
	}
}

type getDirFileContentRequest struct {
	PathStr  string `json:"path_str" binding:"required"`
	Username string `json:"username" binding:"required"`
}

type getDirFileContentResponse struct {
	IsDir   bool         `json:"is_dir"`
	FileStr string       `json:"file_str"`
	DirList []dirContent `json:"dir_list"`
}

func (server *Server) GetDirFileContent(ctx *gin.Context) {
	var req getDirFileContentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	file := req.PathStr

	// authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// if authPayload.Email != req.Username {
	// 	err := errors.New("User not authorized to access file/directory.")
	// 	ctx.JSON(http.StatusBadRequest, errorResponse(err))
	// 	return
	// }

	// file
	// filePath := server.config.BinPath + "/" + authPayload.Username + "/" + file
	filePath := "/" + file

	info, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	var res getDirFileContentResponse
	if info.IsDir() {
		dirs, err := ioutil.ReadDir(filePath)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}

		var dirList []dirContent
		const layoutTime = "2006-01-02 15:04:05"
		for id, dir := range dirs {
			if !strings.HasPrefix(dir.Name(), ".") {
				dirList = append(dirList, dirContent{
					Id:       id,
					Filename: dir.Name(),
					IsDir:    dir.IsDir(),
					Size:     dir.Size(),
					Path:     req.PathStr + "/" + dir.Name(),
					ModTime:  dir.ModTime().Format(layoutTime),
				})
			}
		}
		res.IsDir = true
		res.DirList = dirList

	} else {
		fileString, err := ioutil.ReadFile(filePath)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}
		res.IsDir = false
		res.FileStr = strings.Trim(string(fileString), " ")
	}
	ctx.JSON(http.StatusOK, res)
}

type getDirContentRequest struct {
	PathStr string `json:"path_str" binding:"required"`
}

func (server *Server) GetDirContent(ctx *gin.Context) {
	var req getDirContentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// directory path
	dirPath := "/" + req.PathStr
	dirs, err := ioutil.ReadDir(dirPath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
	}

	var res []dirContent
	const layoutTime = "2006-01-02 15:04:05"
	for id, dir := range dirs {
		if !strings.HasPrefix(dir.Name(), ".") {
			res = append(res, dirContent{
				Id:       id,
				Filename: dir.Name(),
				IsDir:    dir.IsDir(),
				Size:     dir.Size(),
				Path:     req.PathStr + "/" + dir.Name(),
				ModTime:  dir.ModTime().Format(layoutTime),
			})
		}
	}
	ctx.JSON(http.StatusOK, res)
}
