package api

import (
	"fmt"

	db2 "github.com/diantanjung/wecom/db/sqlc"
	"github.com/diantanjung/wecom/token"
	"github.com/diantanjung/wecom/util"
	"github.com/gin-gonic/gin"
)

type Server struct {
	config     util.Config
	querier    db2.Querier
	tokenMaker token.Maker
	router     *gin.Engine
}

// NewServer creates a new HTTP server and set up routing.
func NewServer(config util.Config, querier db2.Querier) (*Server, error) {
	tokenMaker, err := token.NewJWTMaker(config.TokenSymmetricKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create token maker: %w", err)
	}

	server := &Server{
		config:     config,
		querier:    querier,
		tokenMaker: tokenMaker,
	}

	server.setupRouter()
	return server, nil
}

func (server *Server) CORSMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("Access-Control-Allow-Origin", server.config.FeUrl)
		ctx.Header("Access-Control-Allow-Credentials", "true")
		ctx.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		ctx.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if ctx.Request.Method == "OPTIONS" {
			ctx.AbortWithStatus(200)
		} else {
			ctx.Next()
		}
	}
}

func (server *Server) setupRouter() {
	router := gin.Default()

	router.Use(server.CORSMiddleware())

	router.POST("/users/login-google", server.loginGoogle)
	router.GET("/ws2/:username", server.WebSocket2)

	router.POST("/run", server.RunCommand)
	router.GET("/runfunc", server.RunFunc)

	// for guest
	router.POST("/gopendirfile", server.GetDirFileContent)
	router.POST("/gopendir", server.GetDirContent)

	router.PATCH("/open", server.UpdateFileContent)

	authRoutes := router.Group("/").Use(authMiddleware(server.tokenMaker))
	authRoutes.GET("/user", server.getUser)

	authRoutes.POST("/open", server.GetFileContent)

	authRoutes.POST("/opendirfile", server.GetDirFileContent)
	authRoutes.POST("/opendir", server.GetDirContent)

	server.router = router
}

// Start runs the HTTP server on a specific address.
func (server *Server) Start(address string) error {
	return server.router.Run(address)
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}
