package api

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type runGodefRequest struct {
	PathStr  string `json:"path_str" binding:"required"`
	Offset   int    `json:"offset" binding:"required"`
	Username string `json:"username" binding:"required"`
}

type runGodefResponse struct {
	FileStr    string `json:"file_str"`
	Filepath   string `json:"filepath"`
	Dirpath    string `json:"dirpath"`
	Language   string `json:"language"`
	LineNumber int    `json:"line_number"`
	Column     int    `json:"column"`
}

// todo : jalankan godef. contoh : godef -f main.go -o 65.
func (server *Server) RunGodef(ctx *gin.Context) {
	var req runGodefRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// if authPayload.Email != req.Username {
	// 	err := errors.New("User not authorized to access file/directory.")
	// 	ctx.JSON(http.StatusBadRequest, errorResponse(err))
	// 	return
	// }

	// file
	// filePath := server.config.BinPath + "/" + authPayload.Username + "/" + file
	pathFile := req.PathStr

	info, err := os.Stat(pathFile)
	if os.IsNotExist(err) {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if info.IsDir() {
		err = errors.New("File path is a directory.")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	// todo : run godef
	if fileInfo, err := os.Stat(pathFile); err != nil || fileInfo.IsDir() {
		err = errors.New("Command or file not found.")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	offset := strconv.Itoa(req.Offset)

	exeCmd := exec.Command("godef", "-f", pathFile, "-o", offset)
	exeCmd.Dir = filepath.Dir(pathFile)
	var out bytes.Buffer
	var stderr bytes.Buffer
	exeCmd.Stdout = &out
	exeCmd.Stderr = &stderr
	err = exeCmd.Run()

	if err != nil {
		if len(stderr.String()) > 0 {
			err = errors.New(stderr.String())
		}

		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	regex := regexp.MustCompile(`(.+):([0-9]+):([0-9]+)`)
	match := regex.FindStringSubmatch(out.String())
	if len(match) < 4 {
		err = errors.New("godef : not found definition")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	resPath := match[1]

	fileString, err := ioutil.ReadFile(resPath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	var res runGodefResponse
	res.FileStr = strings.Trim(string(fileString), " ")
	res.Filepath, err = filepath.Abs(resPath)
	res.Dirpath = filepath.Dir(resPath)
	ext := filepath.Ext(resPath)
	if ext == ".go" {
		res.Language = "go"
	} else if ext == ".rkt" {
		res.Language = "racket"
	} else if ext == ".rs" {
		res.Language = "rust"
	} else {
		res.Language = "go"
	}

	if res.LineNumber, err = strconv.Atoi(match[2]); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	if res.Column, err = strconv.Atoi(match[3]); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusOK, res)
}
