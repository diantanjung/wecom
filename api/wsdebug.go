package api

import (
	"bufio"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 8192

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Time to wait before force close on connection.
	closeGracePeriod = 10 * time.Second
)

//var username = "";
//var curPath = "";

func (server *Server) pumpStdin(ws *websocket.Conn, w io.Writer) {
	defer ws.Close()
	ws.SetReadLimit(maxMessageSize)
	ws.SetReadDeadline(time.Now().Add(pongWait))
	ws.SetPongHandler(func(string) error { ws.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := ws.ReadMessage()
		if err != nil {
			break
		}
		message = append(message, '\n')
		if _, err := w.Write(message); err != nil {
			break
		}
	}
}

func pumpStdout(ws *websocket.Conn, r io.Reader, done chan struct{}) {
	defer func() {
	}()
	s := bufio.NewScanner(r)
	for s.Scan() {
		ws.SetWriteDeadline(time.Now().Add(writeWait))
		if err := ws.WriteMessage(websocket.TextMessage, s.Bytes()); err != nil {
			ws.Close()
			break
		}
	}
	if s.Err() != nil {
		log.Println("scan:", s.Err())
	}
	close(done)

	ws.SetWriteDeadline(time.Now().Add(writeWait))
	ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	time.Sleep(closeGracePeriod)
	ws.Close()
}

func ping(ws *websocket.Conn, done chan struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(writeWait)); err != nil {
				log.Println("ping:", err)
			}
		case <-done:
			return
		}
	}
}

func internalError(ws *websocket.Conn, msg string, err error) {
	log.Println(msg, err)
	ws.WriteMessage(websocket.TextMessage, []byte("Internal server error."))
}

func (server *Server) WsDebug(ctx *gin.Context) {
	opts := HandlerOpts{
		AllowedHostnames:     []string{"localhost", "bilang.io"},
		Arguments:            []string{},
		Command:              "/bin/bash",
		ConnectionErrorLimit: 10,
		KeepalivePingTimeout: 20,
		MaxBufferSizeBytes:   1024,
	}

	connectionErrorLimit := opts.ConnectionErrorLimit
	if connectionErrorLimit < 0 {
		connectionErrorLimit = DefaultConnectionErrorLimit
	}
	maxBufferSizeBytes := opts.MaxBufferSizeBytes
	keepalivePingTimeout := opts.KeepalivePingTimeout
	if keepalivePingTimeout <= time.Second {
		keepalivePingTimeout = 20 * time.Second
	}

	allowedHostnames := opts.AllowedHostnames
	upgrader := getConnectionUpgrader(allowedHostnames, maxBufferSizeBytes)
	ws, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		log.Println("failed to upgrade connection: %s", err)
		return
	}

	// var upgrader = websocket.Upgrader{}
	// upgrader = websocket.Upgrader{
	// 	ReadBufferSize:  1024,
	// 	WriteBufferSize: 1024,
	// 	// Resolve cross-domain problems
	// 	CheckOrigin: func(r *http.Request) bool {
	// 		return true
	// 	},
	// }

	// ws, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	// if err != nil {
	// 	log.Println("upgrade:", err)
	// 	return
	// }

	defer ws.Close()

	outr, outw, err := os.Pipe()
	if err != nil {
		internalError(ws, "stdout:", err)
		return
	}
	defer outr.Close()
	defer outw.Close()

	inr, inw, err := os.Pipe()
	if err != nil {
		internalError(ws, "stdin:", err)
		return
	}
	defer inr.Close()
	defer inw.Close()

	cmdPath, err := exec.LookPath("/bin/bash")
	if err != nil {
		internalError(ws, "lookPath:", err)
		return
	}

	proc, err := os.StartProcess(cmdPath, flag.Args(), &os.ProcAttr{
		//Dir: wDir,
		Files: []*os.File{inr, outw, outw},
	})
	if err != nil {
		internalError(ws, "start:", err)
		return
	}

	inr.Close()
	outw.Close()

	stdoutDone := make(chan struct{})
	go pumpStdout(ws, outr, stdoutDone)
	go ping(ws, stdoutDone)

	server.pumpStdin(ws, inw)

	// Some commands will exit when stdin is closed.
	inw.Close()

	// Other commands need a bonk on the head.
	if err := proc.Signal(os.Interrupt); err != nil {
		log.Println("inter:", err)
	}

	select {
	case <-stdoutDone:
	case <-time.After(time.Second):
		// A bigger bonk on the head.
		if err := proc.Signal(os.Kill); err != nil {
			log.Println("term:", err)
		}
		<-stdoutDone
	}

	if _, err := proc.Wait(); err != nil {
		log.Println("wait:", err)
	}
}
