package api

import (
	"bytes"
	"errors"
	"github.com/creack/pty"
	db "github.com/diantanjung/wecom/db/sqlc"
	"github.com/diantanjung/wecom/token"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

type commandResponse struct {
	Path    string `json:"path"`
	Command string `json:"command"`
	Message string `json:"message"`
}

func (server *Server) ExeCommand(ctx *gin.Context) {
	//if !server.isUserHasDir(ctx) {
	//	err := errors.New("Directory doesn't belong to the authenticated user")
	//	ctx.JSON(http.StatusUnauthorized, errorResponse(err))
	//	return
	//}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	dir := "/" + ctx.Param("dir") + "/"
	path := server.config.BinPath + "/" + authPayload.Username + dir
	runner := path + ctx.Param("cmd")

	var args []string
	query := ctx.Request.URL.Query()

	for key, val := range query {
		args = append(args, "-"+key)
		args = append(args, val[0])
	}

	cmd := exec.Command(runner, args[0:]...)
	cmd.Dir = path
	stdout, err := cmd.Output()

	command := runner + " " + strings.Join(args, " ")

	var message string
	if err != nil {
		message = "Error! " + err.Error()
	} else {
		message = "Success! " + string(stdout)
	}
	res := commandResponse{
		Path:    path,
		Command: command,
		Message: message,
	}

	ctx.JSON(http.StatusOK, res)
}

type getSourceCodeResponse struct {
	MainStr  string `json:"main_str"`
	GoModStr string `json:"gomod_str"`
}

func (server *Server) GetSourceCode(ctx *gin.Context) {
	//if !server.isUserHasDir(ctx) {
	//	err := errors.New("Directory doesn't belong to the authenticated user")
	//	ctx.JSON(http.StatusUnauthorized, errorResponse(err))
	//	return
	//}
	dir := ctx.Param("dir")
	cmd := ctx.Param("cmd")

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	// main.go file
	mainFile := server.config.BinPath + "/" + authPayload.Username + "/" + dir + "/" + cmd + ".src/main.go"
	dirPath := server.config.BinPath + "/" + authPayload.Username + "/" + dir
	mainStr, err := ioutil.ReadFile(mainFile)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	//go.mod file
	goModStr, err := ioutil.ReadFile(dirPath + "/" + cmd + ".src/go.mod")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	res := getSourceCodeResponse{
		MainStr:  strings.Trim(string(mainStr), " "),
		GoModStr: strings.Trim(string(goModStr), " "),
	}
	ctx.JSON(http.StatusOK, res)
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
	filePath :=  "/" + file
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

type updateSourceCodeRequest struct {
	MainStr  string `json:"main_str" binding:"required"`
	GoModStr string `json:"gomod_str" binding:"required"`
}

func (server *Server) UpdateSourceCode(ctx *gin.Context) {
	//if !server.isUserHasDir(ctx) {
	//	err := errors.New("Directory doesn't belong to the authenticated user")
	//	ctx.JSON(http.StatusUnauthorized, errorResponse(err))
	//	return
	//}
	var req updateSourceCodeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	dir := ctx.Param("dir")
	cmd := ctx.Param("cmd")

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	mainFile := server.config.BinPath + "/" + authPayload.Username + "/" + dir + "/" + cmd + ".src/main.go"
	dirPath := server.config.BinPath + "/" + authPayload.Username + "/" + dir

	err := ioutil.WriteFile(mainFile, []byte(req.MainStr), 0644)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}
	err = ioutil.WriteFile(dirPath+"/"+cmd+".src/go.mod", []byte(req.GoModStr), 0644)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	exeCmd := exec.Command("/usr/local/go/bin/go", "build", "-o", cmd, mainFile)
	exeCmd.Dir = dirPath
	stdout, err := exeCmd.Output()

	var message string
	if err != nil {
		message = "Error! " + err.Error()
	} else {
		message = "Success! " + string(stdout)
	}
	res := commandResponse{
		Message: message,
	}

	ctx.JSON(http.StatusOK, res)
}

func (server *Server) CreateCommand(ctx *gin.Context) {
	dir := ctx.Param("dir")
	cmd := ctx.Param("cmd")
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	// mkdir newdir ; cp -r master/main.src newdir/newcommand.src ; cp master/main newdir/newcommand
	exeCmd := exec.Command("mkdir", dir)
	exeCmd.Dir = server.config.BinPath + "/" + authPayload.Username
	_, _ = exeCmd.Output()

	exeCmd = exec.Command("cp", "-r", "master/main.src", dir+"/"+cmd+".src")
	exeCmd.Dir = server.config.BinPath + "/" + authPayload.Username
	_, _ = exeCmd.Output()

	exeCmd = exec.Command("cp", "master/main", dir+"/"+cmd)
	exeCmd.Dir = server.config.BinPath + "/" + authPayload.Username
	_, _ = exeCmd.Output()

	if !server.isUserHasDir(ctx) {
		authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
		req := db.CreateUserDirParams{
			UserID: authPayload.UserID,
			Name:   dir,
		}
		_, err := server.querier.CreateUserDir(ctx, req)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}
	}

	res := commandResponse{
		Message: "Command successfuly created!",
	}

	ctx.JSON(http.StatusOK, res)
}

func (server *Server) DeleteCommand(ctx *gin.Context) {
	//if !server.isUserHasDir(ctx) {
	//	err := errors.New("Directory doesn't belong to the authenticated user")
	//	ctx.JSON(http.StatusUnauthorized, errorResponse(err))
	//	return
	//}
	dir := ctx.Param("dir")
	cmd := ctx.Param("cmd")

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	exeCmd := exec.Command("rm", "-r", dir+"/"+cmd+".src")
	exeCmd.Dir = server.config.BinPath + "/" + authPayload.Username
	_, _ = exeCmd.Output()

	exeCmd = exec.Command("rm", dir+"/"+cmd)
	exeCmd.Dir = server.config.BinPath + "/" + authPayload.Username
	_, _ = exeCmd.Output()

	res := commandResponse{
		Message: "Command successfuly deleted!",
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

type getCommandsResponse struct {
	Dir string `json:"dir"`
	Cmd string `json:"cmd"`
}

func (server *Server) GetCommands(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	userDirs, err := server.querier.GetUserDirs(ctx, authPayload.UserID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
	}

	userDirsStr := "|"

	for _, dir := range userDirs {
		userDirsStr += dir.Name + "|"
	}

	res := []getCommandsResponse{}
	dirs, err := ioutil.ReadDir(server.config.BinPath + "/" + authPayload.Username)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
	}

	//var strCommand string
	var cmd, dirStr string
	for _, dir := range dirs {
		if dir.IsDir() && strings.Index(userDirsStr, "|"+dir.Name()+"|") > -1 {
			files, _ := ioutil.ReadDir(server.config.BinPath + "/" + authPayload.Username + "/" + dir.Name())
			dirStr = dir.Name()
			for _, file := range files {
				if !file.IsDir() && strings.Index(file.Name(), ".") < 0 {
					cmd = file.Name()
					if cmd != "" && dirStr != "" {
						res = append(res, getCommandsResponse{Cmd: cmd, Dir: dirStr})
					}
				}
			}
		}
	}

	ctx.JSON(http.StatusOK, res)
}

//func (server *Server) Terminal(ctx *gin.Context) {
//	if !server.isUserHasDir(ctx) {
//		err := errors.New("Directory doesn't belong to the authenticated user")
//		ctx.JSON(http.StatusUnauthorized, errorResponse(err))
//		return
//	}
//
//	dir := ctx.Param("dir")
//	cmd := ctx.Param("cmd")
//	exe := ctx.Param("exe")
//	mainFile := server.config.BinPath + "/" + authPayload.Username + "/" + dir + "/" + cmd + ".src/main.go"
//	dirRunner := server.config.BinPath + "/" + authPayload.Username + "/" + dir
//	dirSrc := server.config.BinPath + "/" + authPayload.Username + "/" + dir + "/" + cmd + ".src"
//
//	var commandStr string
//	var commandDir string
//	var args []string
//
//	switch exe {
//	case "ls":
//		commandStr = "ls"
//		commandDir = dirSrc
//	case "code":
//		commandStr = "ls"
//		commandDir = dirSrc
//	case "build":
//		commandStr = "/usr/local/go/bin/go"
//		args = append(args,"build", "-o", ctx.Param("cmd"), mainFile)
//		commandDir = dirRunner
//	case "run":
//		commandStr = dirRunner + "/" + cmd
//		commandDir = dirRunner
//	default:
//		ctx.JSON(http.StatusOK, commandResponse{Message: "Command not found. Try running `help`."})
//	}
//
//	exeCmd := exec.Command(commandStr, args[0:]...)
//	exeCmd.Dir = commandDir
//	stdout, err := exeCmd.Output()
//
//	var message string
//	if err != nil {
//		message = err.Error()
//	} else {
//		if len(stdout) > 0 {
//			message = string(stdout)
//		}else{
//			message = "Succes to execute `" + exe + "`"
//		}
//
//	}
//	res := commandResponse{
//		Message: message,
//	}
//
//	ctx.JSON(http.StatusOK, res)
//}

type terminalRequest struct {
	Exe  string `json:"exe" binding:"required"`
	Path string `json:"path" binding:"required"`
}

func (server *Server) Terminal(ctx *gin.Context) {
	var req terminalRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	rgxExe := regexp.MustCompile(` {2,}`)
	req.Exe = rgxExe.ReplaceAllString(req.Exe, " ")
	req.Exe = strings.Trim(req.Exe, " ")

	// exeArr := strings.Split(req.Exe, " ")

	r := regexp.MustCompile("'.+'|\".+\"|\\S+")
	exeArr := r.FindAllString(req.Exe, -1)

	//if len(exeArr) > 2 {
	//	err := errors.New("Format command not found.")
	//	ctx.JSON(http.StatusBadRequest, errorResponse(err))
	//	return
	//}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	reqPath := req.Path
	reqPath = strings.Replace(reqPath, "~", "/home/"+authPayload.Username, -1)
	reqPathArr := strings.Split(reqPath, "/")
	lenPathArr := len(reqPathArr)
	fullPath := server.config.BinPath + "/" + authPayload.Username + reqPath

	message := ""
	pathStr := ""

	newUser, err := user.Lookup(authPayload.Username)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	uid, err := strconv.ParseUint(newUser.Uid, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	auid := uint32(uid)

	gid, err := strconv.ParseUint(newUser.Gid, 10, 32)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	agid := uint32(gid)

	switch exeArr[0] {
	case "ls":
		//if len(exeArr) == 2 {
		//	if exeArr[1] == ".." && reqPath == "/" {
		//		exeArr[1] = "/"
		//	}
		//}

		rgxPath := regexp.MustCompile(`\.\.`)

		for i, val := range exeArr {
			if val[0:1] == "/" {
				exeArr[i] = server.config.BinPath + "/" + authPayload.Username + val
			}

			exeArr[i] = strings.Replace(exeArr[i], "~", server.config.BinPath+"/"+authPayload.Username+"/home/"+authPayload.Username+"/", -1)

			//if val == "~" {
			//	exeArr[i] = server.config.BinPath + "/" + authPayload.Username + "/home/" + authPayload.Username
			//}

			if rgxPath.MatchString(val) {
				pathTemp := path.Join(fullPath+"/", val)
				if !strings.Contains(pathTemp, server.config.BinPath+"/"+authPayload.Username) {
					err := errors.New("Location not found.")
					ctx.JSON(http.StatusBadRequest, errorResponse(err))
					return
				}
			}
		}
		arguments := append([]string{"-F"}, exeArr[1:]...)
		exeCmd := exec.Command("ls", arguments...)
		//exeCmd := exec.Command("ls",exeArr[1:]...)
		exeCmd.Dir = fullPath

		f, err := pty.Start(exeCmd)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		var buf bytes.Buffer
		_, err = buf.ReadFrom(f)
		//if err != nil {
		//	ctx.JSON(http.StatusBadRequest, errorResponse(err))
		//	return
		//}

		strOut := strings.Replace(buf.String(), "\\t", "\\t ", -1)

		rgxDir := regexp.MustCompile(`([[:graph:]]+)/`)

		rgxBin := regexp.MustCompile(`([[:graph:]]+)\*`)

		strOut = rgxDir.ReplaceAllString(strOut, "\u001B[1;34m$1\u001B[0m")

		strOut = rgxBin.ReplaceAllString(strOut, "\u001B[1;31m$1\u001B[0m")

		message = strOut
	case "cd":
		if len(exeArr) != 2 {
			err := errors.New("Format command not found.")
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		if exeArr[1] == "" {
			err := errors.New("")
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		rgx := regexp.MustCompile(`/{2,}`)
		exeArr[1] = rgx.ReplaceAllString(exeArr[1], "/")

		rgx = regexp.MustCompile(`(.)/+$`)
		exeArr[1] = rgx.ReplaceAllString(exeArr[1], "$1")

		isDot := false
		for _, val := range exeArr[1] {
			if val == 46 {
				isDot = true
			} else {
				isDot = false
			}
		}

		cdPath := ""
		lenDot := len(exeArr[1])
		if isDot && lenDot > 1 {
			if lenPathArr < lenDot {
				err := errors.New("Directory not found")
				ctx.JSON(http.StatusBadRequest, errorResponse(err))
				return
			}
			joinStr := strings.Join(reqPathArr[1:lenPathArr-(lenDot-1)], "/")
			if joinStr != "" {
				cdPath = "/" + joinStr
			}
		} else {
			if exeArr[1][0:1] == "~" {
				cdPath = "/home/" + authPayload.Username + exeArr[1][1:]
			} else if exeArr[1][0:1] == "/" {
				cdPath = exeArr[1]
			} else {
				lenReqPath := len(reqPath)
				if reqPath[lenReqPath-1:lenReqPath] == "/" {
					cdPath = reqPath + exeArr[1]
				} else {
					cdPath = reqPath + "/" + exeArr[1]
				}

			}
		}
		if cdPath != "" {
			if fileInfo, err := os.Stat(server.config.BinPath + "/" + authPayload.Username + cdPath); err != nil || !fileInfo.IsDir() {
				err := errors.New("Directory not found.")
				ctx.JSON(http.StatusBadRequest, errorResponse(err))
				return
			}
			message = strings.Replace(cdPath, "/home/"+authPayload.Username, "~", -1)

		} else {
			message = "/"
		}
	case "go":
		exeCmd := exec.Command(server.config.GoBinPath, exeArr[1:]...)
		exeCmd.Dir = fullPath
		var out bytes.Buffer
		var stderr bytes.Buffer
		exeCmd.Stdout = &out
		exeCmd.Stderr = &stderr
		err := exeCmd.Run()
		if err != nil {
			message = stderr.String()
		} else {
			message = out.String()
		}
	case "cargo":
		exeCmd := exec.Command(server.config.CargoBinPath, exeArr[1:]...)
		exeCmd.Dir = fullPath
		var out bytes.Buffer
		var stderr bytes.Buffer
		exeCmd.Stdout = &out
		exeCmd.Stderr = &stderr
		err := exeCmd.Run()
		if err != nil {
			message = stderr.String()
		} else {
			message = out.String()
		}
	case "open":
		if len(exeArr) < 2 {
			err := errors.New("Format command not found.")
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}
		filePath := reqPath + "/" + exeArr[1]

		//fileInfo, err := os.Stat(server.config.BinPath + "/" + authPayload.Username + filePath)
		//if err != nil {
		//	_, err := os.Create(server.config.BinPath + "/" + authPayload.Username + filePath)
		//	if err != nil {
		//		err := errors.New("Write file error. Check permission")
		//		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		//		return
		//	}
		//}
		//if fileInfo != nil && fileInfo.IsDir() {
		//	pathStr = "/opendir" + filePath
		//} else if !strings.Contains(fileInfo.Name(), ".") {
		//	pathStr = "/run" + filePath
		//} else {
		//	//pathStr = "<a href=\"editcode/" + dir + "/" + cmd + "\" >Edit command" + dir + "/" + cmd + " </a>"
		//	pathStr = "/editfile" + filePath
		//}
		pathStr = "/@" + authPayload.Username + filePath
	case "adduser":
		pathStr = "/adduser"
	case "mkdir":
		exeCmd := exec.Command("mkdir", exeArr[1:]...)
		exeCmd.Dir = fullPath
		var out bytes.Buffer
		var stderr bytes.Buffer
		exeCmd.Stdout = &out
		exeCmd.Stderr = &stderr
		exeCmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: auid, Gid: agid}}
		err := exeCmd.Run()
		if err != nil {
			message = stderr.String()
		} else {
			message = out.String()
		}
	case "rm":
		exeCmd := exec.Command("rm", exeArr[1:]...)
		exeCmd.Dir = fullPath
		var out bytes.Buffer
		var stderr bytes.Buffer
		exeCmd.Stdout = &out
		exeCmd.Stderr = &stderr
		err := exeCmd.Run()
		if err != nil {
			message = stderr.String()
		} else {
			message = out.String()
		}
	case "rmdir":
		exeCmd := exec.Command("rmdir", exeArr[1:]...)
		exeCmd.Dir = fullPath
		var out bytes.Buffer
		var stderr bytes.Buffer
		exeCmd.Stdout = &out
		exeCmd.Stderr = &stderr
		err := exeCmd.Run()
		if err != nil {
			message = stderr.String()
		} else {
			message = out.String()
		}
	case "touch":
		exeCmd := exec.Command("touch", exeArr[1:]...)
		exeCmd.Dir = fullPath
		var out bytes.Buffer
		var stderr bytes.Buffer
		exeCmd.Stdout = &out
		exeCmd.Stderr = &stderr
		exeCmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: auid, Gid: agid}}
		err := exeCmd.Run()
		if err != nil {
			message = stderr.String()
		} else {
			message = out.String()
		}
	case "ln":
		exeCmd := exec.Command("ln", exeArr[1:]...)
		exeCmd.Dir = fullPath
		var out bytes.Buffer
		var stderr bytes.Buffer
		exeCmd.Stdout = &out
		exeCmd.Stderr = &stderr
		exeCmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: auid, Gid: agid}}
		err := exeCmd.Run()
		if err != nil {
			message = stderr.String()
		} else {
			message = out.String()
		}
	case "cat":
		exeCmd := exec.Command("cat", exeArr[1:]...)
		exeCmd.Dir = fullPath
		var out bytes.Buffer
		var stderr bytes.Buffer
		exeCmd.Stdout = &out
		exeCmd.Stderr = &stderr
		exeCmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: auid, Gid: agid}}
		err := exeCmd.Run()
		if err != nil {
			message = stderr.String()
		} else {
			message = out.String()
		}
	case "pwd":
		exeCmd := exec.Command("pwd")
		exeCmd.Dir = fullPath
		var out bytes.Buffer
		var stderr bytes.Buffer
		exeCmd.Stdout = &out
		exeCmd.Stderr = &stderr
		err := exeCmd.Run()
		if err != nil {
			message = stderr.String()
		} else {
			strCut := strings.Replace(out.String(), server.config.BinPath+"/"+authPayload.Username, "", -1)
			if strCut == "\n" {
				message = "/"
			} else {
				message = strCut
			}
		}
	case "cp":
		exeCmd := exec.Command("cp", exeArr[1:]...)
		exeCmd.Dir = fullPath
		var out bytes.Buffer
		var stderr bytes.Buffer
		exeCmd.Stdout = &out
		exeCmd.Stderr = &stderr
		exeCmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: auid, Gid: agid}}
		err := exeCmd.Run()
		if err != nil {
			message = stderr.String()
		} else {
			message = out.String()
		}
	case "mv":
		exeCmd := exec.Command("mv", exeArr[1:]...)
		exeCmd.Dir = fullPath
		var out bytes.Buffer
		var stderr bytes.Buffer
		exeCmd.Stdout = &out
		exeCmd.Stderr = &stderr
		exeCmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: auid, Gid: agid}}
		err := exeCmd.Run()
		if err != nil {
			message = stderr.String()
		} else {
			message = out.String()
		}
	case "git":
		exeCmd := exec.Command("git", exeArr[1:]...)
		exeCmd.Dir = fullPath
		var out bytes.Buffer
		var stderr bytes.Buffer
		// exeCmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: auid, Gid: agid}}
		exeCmd.Stdout = &out
		exeCmd.Stderr = &stderr
		err := exeCmd.Run()
		if err != nil {
			message = stderr.String()
		} else {
			message = out.String()
		}
	case "nano":
		exeCmd := exec.Command("nano")
		exeCmd.Dir = fullPath
		// var out bytes.Buffer
		// var stderr bytes.Buffer
		// // exeCmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: auid, Gid: agid}}
		// exeCmd.Stdout = &out
		// exeCmd.Stderr = &stderr
		err := exeCmd.Run()
		if err != nil {
			message = err.Error()
		} else {
			message = "succes"
		}
	default:
		filePath := reqPath + "/" + exeArr[0]
		if fileInfo, err := os.Stat(server.config.BinPath + "/" + authPayload.Username + filePath); err != nil || fileInfo.IsDir() {
			err = errors.New("Format command not found.")
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		var args []string
		runnerArr := strings.Split(filePath, "/")
		runnerDir := strings.Join(runnerArr[:(len(runnerArr)-1)], "/")

		if len(exeArr) > 1 {
			args = exeArr[2:]
		}
		// newUser, err := user.Lookup(authPayload.Username)
		// if err != nil {
		// 	ctx.JSON(http.StatusBadRequest, errorResponse(err))
		// 	return
		// }
		// uid, err := strconv.ParseUint(newUser.Uid, 10, 32)
		// if err != nil {
		// 	ctx.JSON(http.StatusBadRequest, errorResponse(err))
		// 	return
		// }
		// auid := uint32(uid)

		// gid, err := strconv.ParseUint(newUser.Gid, 10, 32)
		// if err != nil {
		// 	ctx.JSON(http.StatusBadRequest, errorResponse(err))
		// 	return
		// }

		// agid := uint32(gid)

		//groups,err := newUser.GroupIds()
		//
		//grid, err := strconv.ParseUint(groups[0],10,32)
		//if err != nil {
		//	ctx.JSON(http.StatusBadRequest, errorResponse(err))
		//	return
		//}
		//
		//agrid := uint32(grid)

		exeCmd := exec.Command(filePath, args[0:]...)
		exeCmd.SysProcAttr = &syscall.SysProcAttr{Chroot: server.config.BinPath + "/" + authPayload.Username, Credential: &syscall.Credential{Uid: auid, Gid: agid}}
		exeCmd.Dir = runnerDir
		var out bytes.Buffer
		var stderr bytes.Buffer
		exeCmd.Stdout = &out
		exeCmd.Stderr = &stderr

		_ = exeCmd.Run()

		if len(stderr.String()) > 0 {
			err = errors.New(stderr.String())
			ctx.JSON(http.StatusBadRequest, errorResponse(err))
			return
		}

		if len(out.String()) > 0 {
			message = out.String()
		}
	}

	res := commandResponse{
		Message: message,
		Path:    pathStr,
	}

	ctx.JSON(http.StatusOK, res)
	return
}

func (server *Server) Chroot(path string) (func() error, error) {
	root, err := os.Open("/")
	if err != nil {
		return nil, err
	}

	if err := syscall.Chroot(path); err != nil {
		root.Close()
		return nil, err
	}

	return func() error {
		defer root.Close()
		if err := root.Chdir(); err != nil {
			return err
		}
		return syscall.Chroot(".")
	}, nil
}

type autoCompleteRequest struct {
	Term string `json:"term"`
	Path string `json:"path" binding:"required"`
}

type autoCompleteResponse struct {
	Colored []string `json:"colored"`
	Pure    []string `json:"pure"`
	Rest    string   `json:"rest"`
}

func (server *Server) AutoComplete(ctx *gin.Context) {
	var req autoCompleteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	spaceIdx := strings.LastIndex(req.Term, " ")
	termAuto := ""
	cmdAuto := ""
	if spaceIdx > -1 {
		termAuto = req.Term[spaceIdx+1:]
		cmdAuto = req.Term[:spaceIdx]
	} else {
		termAuto = req.Term
	}

	slashIdx := strings.LastIndex(termAuto, "/")
	termPath := ""
	termRest := ""
	if slashIdx > -1 {
		termPath = termAuto[:slashIdx+1]
		termRest = termAuto[slashIdx+1:]
	} else {
		termRest = termAuto
	}

	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)

	termRest = strings.Replace(termRest, "~", "/home/"+authPayload.Username, -1)

	var fullPath = ""
	if termPath != "" && termPath[0:1] == "~" {
		termPath = strings.Replace(termPath, "~", "/home/"+authPayload.Username, -1)
		//fullPath = server.config.BinPath + "/" + authPayload.Username + termPath
		fullPath = path.Join(server.config.BinPath, authPayload.Username, termPath)
	} else {
		req.Path = strings.Replace(req.Path, "~", "/home/"+authPayload.Username+"/", -1)
		//fullPath = server.config.BinPath + "/" + authPayload.Username + req.Path + termPath
		fullPath = path.Join(server.config.BinPath, authPayload.Username, req.Path, termPath)
	}

	pathTemp := path.Join(server.config.BinPath+"/"+authPayload.Username+req.Path, termPath)
	if !strings.Contains(pathTemp, server.config.BinPath+"/"+authPayload.Username) {
		err := errors.New("Location not found.")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	dirs, err := ioutil.ReadDir(fullPath)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	var res, raw []string
	for _, dir := range dirs {
		if strings.HasPrefix(dir.Name(), termRest) {
			if dir.IsDir() {
				res = append(res, "\u001B[1;34m"+dir.Name()+"/\u001B[0m")
				raw = append(raw, dir.Name()+"/")
			} else {
				if strings.Contains(dir.Name(), ".") {
					res = append(res, "\u001B[1;37m"+dir.Name()+"\u001B[0m")
				} else {
					res = append(res, "\u001B[1;31m"+dir.Name()+"\u001B[0m")
				}
				raw = append(raw, dir.Name())
			}

		}
	}

	if len(res) == 1 && cmdAuto != "" {
		lenRes := len(res[0])
		res[0] = cmdAuto + " " + termPath + res[0][7:lenRes-4]
		raw[0] = cmdAuto + " " + termPath + raw[0]
	}

	if len(res) == 1 && cmdAuto == "" {
		lenRes := len(res[0])
		res[0] = termPath + res[0][7:lenRes-4]
		raw[0] = termPath + raw[0]
	}

	resp := autoCompleteResponse{
		Colored: res,
		Pure:    raw,
		Rest:    termRest,
	}

	ctx.JSON(http.StatusOK, resp)
}

type getDirContentRequest struct {
	PathStr string `json:"path_str" binding:"required"`
}

type dirContent struct {
	Id       int    `json:"id"`
	Filename string `json:"filename"`
	IsDir    bool   `json:"isdir"`
	Size     int64  `json:"size"`
	Path     string `json:"path"`
	ModTime  string `json:"mod_time"`
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

	if authPayload.Username != req.Username {
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
