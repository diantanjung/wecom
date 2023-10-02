package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

type getDirFileContentRequest struct {
	PathStr  string `json:"path_str" binding:"required"`
	Username string `json:"username" binding:"required"`
}

type getDirFileContentResponse struct {
	IsDir    bool         `json:"is_dir"`
	FileStr  string       `json:"file_str"`
	DirList  []dirContent `json:"dir_list"`
	Filepath string       `json:"filepath"`
	Dirpath  string       `json:"dirpath"`
	Language string       `json:"language"`
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
		res.Filepath = filePath
		res.Dirpath = filePath
	} else {
		fileString, err := os.ReadFile(filePath)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}
		res.IsDir = false
		res.FileStr = strings.Trim(string(fileString), " ")
		res.Filepath, err = filepath.Abs(filePath)
		res.Dirpath = filepath.Dir(filePath)
		ext := filepath.Ext(filePath)
		if ext == ".go" {
			res.Language = "go"
		} else if ext == ".rkt" {
			res.Language = "racket"
		} else if ext == ".rs" {
			res.Language = "rust"
		} else {
			res.Language = "go"
		}
	}
	ctx.JSON(http.StatusOK, res)
}

type getCodebaseRequest struct {
	PathStr  string `json:"path_str" binding:"required"`
	Username string `json:"username" binding:"required"`
}

type getCodebaseResponse struct {
	Id       int    `json:"id"`
	Filename string `json:"filename"`
	FileStr  string `json:"file_str"`
	IsDir    bool   `json:"isdir"`
	Filepath string `json:"filepath"`
	Dirpath  string `json:"dirpath"`
	Language string `json:"language"`
	Size     int64  `json:"size"`
	ModTime  string `json:"mod_time"`
}

func (server *Server) GetCodebase(ctx *gin.Context) {
	var req getCodebaseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// directory path
	dirPath := "/" + req.PathStr
	var res []getCodebaseResponse
	const layoutTime = "2006-01-02 15:04:05"
	const limitFiles = 10

	err := filepath.Walk(dirPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			}
			regex := regexp.MustCompile(`/\.`)
			regex_not_binary := regexp.MustCompile(`.+\..+`)
			if !info.IsDir() && !regex.MatchString(path) && regex_not_binary.MatchString(path) {

				fileString, err := os.ReadFile(path)
				if err != nil {
					ctx.JSON(http.StatusInternalServerError, errorResponse(err))
				}

				ext := filepath.Ext(path)
				langfile := "" 
				if ext == ".go" {
					langfile = "go"
				} else if ext == ".rkt" {
					langfile = "racket"
				} else if ext == ".rs" {
					langfile = "rust"
				}

				if langfile != "" {
					res = append(res, getCodebaseResponse{
						Filename: info.Name(),
						IsDir:    info.IsDir(),
						Size:     info.Size(),
						Filepath: path,
						ModTime:  info.ModTime().Format(layoutTime),
						FileStr: strings.Trim(string(fileString), " "),
						Dirpath: filepath.Dir(path),
						Language: langfile,
					})
				}
				
			}
			if len(res) >= limitFiles {
				return filepath.SkipAll
			}

			return nil
		})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
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

type getAllFilesRequest struct {
	PathStr string `json:"path_str" binding:"required"`
	Term    string `json:"term"`
}

type getAllFilesResponse struct {
	Filename string `json:"filename"`
	IsDir    bool   `json:"isdir"`
	Size     int64  `json:"size"`
	Path     string `json:"path"`
	ModTime  string `json:"mod_time"`
}

func (server *Server) GetAllFiles(ctx *gin.Context) {
	var req getAllFilesRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// directory path
	dirPath := "/" + req.PathStr
	var res []getAllFilesResponse
	const layoutTime = "2006-01-02 15:04:05"

	err := filepath.Walk(dirPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			}
			regex := regexp.MustCompile(`/\.`)
			regexTerm := regexp.MustCompile(req.Term)
			regex_not_binary := regexp.MustCompile(`.+\..+`)
			if !info.IsDir() && !regex.MatchString(path) && regexTerm.MatchString(info.Name()) && regex_not_binary.MatchString(path) {
				res = append(res, getAllFilesResponse{
					Filename: info.Name(),
					IsDir:    info.IsDir(),
					Size:     info.Size(),
					Path:     path,
					ModTime:  info.ModTime().Format(layoutTime),
				})
			}

			return nil

		})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
	}

	ctx.JSON(http.StatusOK, res)

}

func (server *Server) RunFunc(ctx *gin.Context) {
	var req runFuncRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	args := make(map[string]string)
	json.Unmarshal([]byte(req.Args), &args)

	fileArr := strings.Split(req.PathStr, "/")
	fileDir := "/" + strings.Join(fileArr[:(len(fileArr)-2)], "/") + "/"
	filePath := "/" + strings.Join(fileArr[:(len(fileArr)-1)], "/")
	fileName := fileArr[(len(fileArr) - 2)]
	funcName := fileArr[(len(fileArr) - 1)]

	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	msg := ""
	functionCall := ""

	isGoFile, _ := regexp.Match("\\.go$", []byte(filePath))
	if isGoFile {
		packageName, _, err := goExtractPackage(fileDir)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		paramFunc, err := goExtractFuncDef(filePath, funcName)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		if len(args) != len(paramFunc) {
			err = errors.New("Not enough arguments in call to function.")
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		argsStr := ""
		for _, v := range paramFunc {
			value, ok := args[v]
			if !ok {
				err = errors.New("Not enough arguments in call to function")
				ctx.JSON(http.StatusBadRequest, errorResponse(err))
				return
			}
			if _, err := strconv.Atoi(value); err != nil {
				argsStr += "\"" + value + "\","
			} else {
				argsStr += value + ","
			}
		}

		if last := len(argsStr) - 1; last >= 0 && argsStr[last] == ',' {
			argsStr = argsStr[:last]
		}

		functionCall = funcName + "(" + argsStr + ")"

		testRandomName := randString(10)
		testFileName := fileDir + testRandomName + "_test.go"
		fileContent := fmt.Sprintf(templateString, packageName, testRandomName, functionCall)
		err = goGenerateAndFmtFile(testFileName, fileDir, fileContent)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		defer os.Remove(testFileName)

		msg, err = goRunFile(testRandomName, testFileName, fileDir)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
	} else {
		isRktFile, _ := regexp.Match("\\.rkt$", []byte(filePath))
		if isRktFile {
			paramFunc, err := rktGetArgs(filePath, funcName)
			if err != nil {
				ctx.JSON(http.StatusBadRequest, errorResponse(err))
				return
			}

			if len(args) != len(paramFunc) {
				err = errors.New("Not enough arguments in call to function.")
				ctx.JSON(http.StatusBadRequest, errorResponse(err))
				return
			}

			argsStr := ""
			for _, v := range paramFunc {
				value, ok := args[v]
				if !ok {
					err = errors.New("Not enough arguments in call to function")
					ctx.JSON(http.StatusBadRequest, errorResponse(err))
					return
				}
				if _, err := strconv.Atoi(value); err != nil {
					argsStr += "\"" + value + "\" "
				} else {
					argsStr += value + " "
				}
			}

			functionCall = funcName + " " + argsStr
			testRandomName := randString(10)
			testFileName := fileDir + testRandomName + ".rkt"
			fileContent := fmt.Sprintf(rktTemplateString, fileName, functionCall)
			err = rktGenerateFile(testFileName, fileContent)
			if err != nil {
				ctx.JSON(http.StatusBadRequest, errorResponse(err))
				return
			}
			defer os.Remove(testFileName)

			msg, err = rktRunFile(testFileName, fileDir)
			if err != nil {
				ctx.JSON(http.StatusBadRequest, errorResponse(err))
				return
			}
		} else {
			isRsFile, _ := regexp.Match("\\.rs$", []byte(filePath))
			if isRsFile {

				paramFunc, err := rsGetArgs(filePath, funcName)
				if err != nil {
					ctx.JSON(http.StatusBadRequest, errorResponse(err))
					return
				}

				if len(args) != len(paramFunc) {
					err = errors.New("Not enough arguments in call to function.")
					ctx.JSON(http.StatusBadRequest, errorResponse(err))
					return
				}

				argsStr := ""
				for _, v := range paramFunc {
					value, ok := args[v]
					if !ok {
						err = errors.New("Not enough arguments in call to function")
						ctx.JSON(http.StatusBadRequest, errorResponse(err))
						return
					}
					if _, err := strconv.Atoi(value); err != nil {
						argsStr += "\"" + value + "\" "
					} else {
						argsStr += value + ","
					}
				}

				if last := len(argsStr) - 1; last >= 0 && argsStr[last] == ',' {
					argsStr = argsStr[:last]
				}

				functionCall = funcName + "(" + argsStr + ")"
				testRandomName := randString(10)
				testFileName := fileDir + testRandomName + ".rs"
				binName := fileName[:len(fileName)-3]
				fileContent := fmt.Sprintf(rsTemplateString, binName, binName, functionCall)
				err = rsGenerateFile(testFileName, fileContent)
				if err != nil {
					ctx.JSON(http.StatusBadRequest, errorResponse(err))
					return
				}
				defer os.Remove(testFileName)
				defer os.Remove(fileDir + testRandomName)

				fmt.Println("testFileName : " + testFileName)
				fmt.Println("testRandomName : " + testRandomName)

				msg, err = rsRunFile(testFileName, testRandomName, fileDir)
				if err != nil {
					ctx.JSON(http.StatusBadRequest, errorResponse(err))
					return
				}

			} else {
				err := errors.New("File's not found.")
				ctx.JSON(http.StatusBadRequest, errorResponse(err))
				return
			}

		}
	}

	res := commandResponse{
		Path:    functionCall,
		Message: msg,
	}

	ctx.JSON(http.StatusOK, res)
}
