package api

import (
	"database/sql"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/diantanjung/wecom/token"

	db "github.com/diantanjung/wecom/db/sqlc"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/oauth2/v2"
)

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

type loginGoogleRequest struct {
	IdToken string `json:"credential" binding:"required"`
}

type loginGoogleResponse struct {
	AccessToken string       `json:"access_token"`
	User        userResponse `json:"user"`
}

func (server *Server) loginGoogle(ctx *gin.Context) {
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

	_, err = server.querier.GetUserByEmail(ctx, tokenInfo.Email)
	username := tokenInfo.Email

	if err != nil {
		if err == sql.ErrNoRows {
			userArr := strings.Split(tokenInfo.Email, "@")

			arg := db.CreateUserParams{
				Username: tokenInfo.Email,
				Password: "-",
				Name:     userArr[0],
				Email:    tokenInfo.Email,
			}
			server.querier.CreateUser(ctx, arg)

			//add user if not available in server
			exec.Command("useradd", "-m", username).Run()
			exec.Command("chmod", "750", "/home/"+username).Run()
			exec.Command("cp", "-r", "/opt/.oh-my-zsh", "/home/"+username+"/.oh-my-zsh").Run()
			exec.Command("chown", "-R", username+":"+username, "/home/"+username+"/.oh-my-zsh").Run()

			exec.Command("cp", "-r", "/opt/.oh-my-zsh/templates/zshrc.zsh-template", "/home/"+username+"/.zshrc").Run()
			exec.Command("chown", "-R", username+":"+username, "/home/"+username+"/.zshrc").Run()

			exec.Command("cp", "-r", "/opt/powerlevel10k", "/home/"+username+"/.powerlevel10k").Run()
			exec.Command("chown", "-R", username+":"+username, "/home/"+username+"/.powerlevel10k").Run()

			exec.Command("cp", "-r", "/opt/.p10k.zsh", "/home/"+username+"/.p10k.zsh").Run()
			exec.Command("chown", "-R", username+":"+username, "/home/"+username+"/.p10k.zsh").Run()

			// install rust 
			// exec.Command("cp", "-r", "/opt/rustup-init", "/home/"+username+"/rustup-init").Run()
			// exec.Command("sudo", "-u", username, "/home/"+username+"/rustup-init", "-y").Run()

			// exec.Command("echo", "'source ~/powerlevel10k/powerlevel10k.zsh-theme'", ">>~/.zshrc").Run()
			// echo 'source ~/powerlevel10k/powerlevel10k.zsh-theme' >>~/.zshrc

			//Append powerlevel theme setting
			file, _ := os.OpenFile("/home/"+username+"/.zshrc", os.O_APPEND|os.O_WRONLY, 0644)
			defer file.Close()
			file.WriteString("source ~/.powerlevel10k/powerlevel10k.zsh-theme\nalias ls='colorls'\nalias logout='quit'\nsudo (){echo sudo: command not found}\nexport PATH=$PATH:/usr/local/go/bin\n[[ ! -f ~/.p10k.zsh ]] || source ~/.p10k.zsh\nexport RUSTUP_HOME=/nfs/rust/rustup\nexport PATH=${PATH}:/nfs/rust/cargo/bin\nexport PATH=${PATH}:/usr/racket/bin")

			exec.Command("usermod", "--shell", "/usr/bin/zsh", username).Run()

			// delete rust 
			// exec.Command("rm", "/home/"+username+"/rustup-init").Run()

			// exec.Command("ln", "-s", "/home/dian/.oh-my-zsh", "/home/" + username + "/.oh-my-zsh").Run()
			// exec.Command("ln", "-s", "/home/dian/.zshrc", "/home/" + username + "/.zshrc").Run()
			// exec.Command("sh", "-c", "$(wget -O- https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)").Run()

		} else {
			ctx.JSON(http.StatusInternalServerError, errorResponse(err))
			return
		}
	}

	userLogin := db.User{
		Username: username,
	}

	rsp := loginGoogleResponse{
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
