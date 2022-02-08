package api

import (
	"database/sql"
	"errors"
	"net/http"
	"os/exec"
	"time"

	"github.com/diantanjung/wecom/token"

	db "github.com/diantanjung/wecom/db/sqlc"
	"github.com/diantanjung/wecom/util"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"google.golang.org/api/oauth2/v2"
)

// var (
// 	googleOauthConfig *oauth2.Config
// 	// TODO: randomize it
// 	oauthStateString = "pseudo-random"
// )

type createUserRequest struct {
	Username string `json:"username" binding:"required,alphanum"`
	Password string `json:"password" binding:"required,min=6"`
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
}

type userResponse struct {
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func newUserResponse(user db.User) userResponse {
	return userResponse{
		Username:  user.Username,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
}

func (server *Server) createUser(ctx *gin.Context) {
	var req createUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	hashedPassword, err := util.HashPassword(req.Password)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	arg := db.CreateUserParams{
		Username: req.Username,
		Password: hashedPassword,
		Name:     req.Name,
		Email:    req.Email,
	}

	user, err := server.querier.CreateUser(ctx, arg)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			switch pqErr.Code.Name() {
			case "unique_violation":
				ctx.JSON(http.StatusForbidden, errorResponse(err))
				return
			}
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	exeCmd := exec.Command("mkdir", user.Username)
	exeCmd.Dir = server.config.BinPath
	_, err = exeCmd.Output()

	if err != nil {
		err := errors.New("Failed to create home directory user.")
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	rsp := newUserResponse(user)
	ctx.JSON(http.StatusOK, rsp)
}

type loginUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginUserResponse struct {
	AccessToken string       `json:"access_token"`
	User        userResponse `json:"user"`
}

func (server *Server) loginUser(ctx *gin.Context) {
	var req loginUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	userLogin, err := server.querier.GetUser(ctx, req.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	err = util.CheckPassword(req.Password, userLogin.Password)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, errorResponse(err))
		return
	}

	accessToken, err := server.tokenMaker.CreateToken(
		userLogin.UserID,
		userLogin.Username,
		server.config.AccessTokenDuration,
	)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	ctx.SetCookie("access_token", accessToken, 3600, "/", "localhost", false, true)

	rsp := loginUserResponse{
		AccessToken: accessToken,
		User:        newUserResponse(userLogin),
	}

	//path := server.config.BinPath + "/" + userLogin.Username
	//
	//newUser, err := user.Lookup(userLogin.Username)
	//if err != nil {
	//	ctx.JSON(http.StatusInternalServerError, errorResponse(err))
	//	return
	//}
	//uid, err := strconv.Atoi(newUser.Uid)
	//if err != nil {
	//	ctx.JSON(http.StatusInternalServerError, errorResponse(err))
	//	return
	//}
	//gid, err := strconv.Atoi(newUser.Gid)
	//if err != nil {
	//	ctx.JSON(http.StatusInternalServerError, errorResponse(err))
	//	return
	//}
	//// Change file ownership.
	//err = os.Chown(path, uid , gid)
	//
	//if err != nil {
	//	ctx.JSON(http.StatusInternalServerError, errorResponse(err))
	//	return
	//}

	//exeCmd := exec.Command("chown","-R" , userLogin.Username, userLogin.Username)
	//exeCmd.Dir = server.config.BinPath
	//err = exeCmd.Run()
	//
	//if err != nil {
	//	ctx.JSON(http.StatusInternalServerError, errorResponse(err))
	//	return
	//}
	//
	//exeCmd = exec.Command("chgrp","-R" , userLogin.Username, userLogin.Username)
	//exeCmd.Dir = server.config.BinPath
	//err = exeCmd.Run()
	//
	//if err != nil {
	//	ctx.JSON(http.StatusInternalServerError, errorResponse(err))
	//	return
	//}

	ctx.JSON(http.StatusOK, rsp)
}

type loginGoogleRequest struct {
	IdToken string `json:"id_token" binding:"required"`
}

type loginGoogleResponse struct {
	AccessToken string       `json:"access_token"`
	User        userResponse `json:"user"`
}

func (server *Server) loginGoogle(ctx *gin.Context) {
	// googleOauthConfig = &oauth2.Config{
	// 	RedirectURL:  "http://localhost:3000/callback",
	// 	ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
	// 	ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
	// 	Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
	// 	Endpoint:     google.Endpoint,
	// }

	// url := googleOauthConfig.AuthCodeURL(oauthStateString)
	// ctx.Redirect(http.StatusTemporaryRedirect, url)

	var req loginGoogleRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	httpClient := &http.Client{}
	oauth2Service, err := oauth2.New(httpClient)
	tokenInfoCall := oauth2Service.Tokeninfo()
	tokenInfoCall.IdToken(req.IdToken)
	tokenInfo, err := tokenInfoCall.Do()
	if err != nil {
		ctx.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}

	userLogin := db.User{
		Username: tokenInfo.Email,
	}

	rsp := loginUserResponse{
		AccessToken: req.IdToken,
		User:        newUserResponse(userLogin),
	}

	ctx.JSON(http.StatusOK, rsp)
}

func (server *Server) getUser(ctx *gin.Context) {
	authPayload := ctx.MustGet(authorizationPayloadKey).(*token.Payload)
	user, err := server.querier.GetUser(ctx, authPayload.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, errorResponse(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, errorResponse(err))
		return
	}

	ctx.JSON(http.StatusOK, newUserResponse(user))
}
