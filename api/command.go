package api

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
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

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	if authPayload.Email != req.Username {
		err := errors.New("User not authorized to access file/directory.")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

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
			dirList = append(dirList, dirContent{
				Id:       id,
				Filename: dir.Name(),
				IsDir:    dir.IsDir(),
				Size:     dir.Size(),
				Path:     req.PathStr + "/" + dir.Name(),
				ModTime:  dir.ModTime().Format(layoutTime),
			})
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
		res = append(res, dirContent{
			Id:       id,
			Filename: dir.Name(),
			IsDir:    dir.IsDir(),
			Size:     dir.Size(),
			Path:     req.PathStr + "/" + dir.Name(),
			ModTime:  dir.ModTime().Format(layoutTime),
		})
	}
	ctx.JSON(http.StatusOK, res)
}
