package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

const DefaultConnectionErrorLimit = 10

type HandlerOpts struct {
	AllowedHostnames     []string
	Arguments            []string
	Command              string
	ConnectionErrorLimit int
	KeepalivePingTimeout time.Duration
	MaxBufferSizeBytes   int
}

var WebsocketMessageType = map[int]string{
	websocket.BinaryMessage: "binary",
	websocket.TextMessage:   "text",
	websocket.CloseMessage:  "close",
	websocket.PingMessage:   "ping",
	websocket.PongMessage:   "pong",
}

type TTYSize struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
	X    uint16 `json:"x"`
	Y    uint16 `json:"y"`
}

func (server *Server) WebSocket2(ctx *gin.Context) {
	// func GetHandler(opts HandlerOpts) func(http.ResponseWriter, *http.Request) {
	opts := HandlerOpts{
		AllowedHostnames:     []string{"localhost", server.config.BaseUrl},
		Arguments:            []string{},
		Command:              "/bin/bash",
		ConnectionErrorLimit: 10,
		KeepalivePingTimeout: 20,
		MaxBufferSizeBytes:   512,
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
	connection, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		fmt.Println("failed to upgrade connection: %s", err)
		return
	}

	username := ctx.Param("username")

	terminal := opts.Command
	args := opts.Arguments
	fmt.Println("starting new tty using command '%s' with arguments ['%s']...", terminal, strings.Join(args, "', '"))
	// cmd := exec.Command(terminal, args...)
	cmd := exec.Command("schroot", "-c", "xenial", "-u", username)
	// sudo schroot -c xenial -u dian
	// cmd := exec.Command("/bin/bash")
	cmd.Dir = "/home/" + username
	// cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 1000, Gid: 1000}}
	// cmd.SysProcAttr = &syscall.SysProcAttr{Chroot: "/var/chroot/xenial"}
	cmd.Env = os.Environ()
	tty, err := pty.Start(cmd)
	if err != nil {
		message := fmt.Sprintf("failed to start tty: %s", err)
		fmt.Println(message)
		connection.WriteMessage(websocket.TextMessage, []byte(message))
		return
	}
	defer func() {
		fmt.Println("gracefully stopping spawned tty...")
		if err := cmd.Process.Kill(); err != nil {
			fmt.Println("failed to kill process: %s", err)
		}
		if _, err := cmd.Process.Wait(); err != nil {
			fmt.Println("failed to wait for process to exit: %s", err)
		}
		if err := tty.Close(); err != nil {
			fmt.Println("failed to close spawned tty gracefully: %s", err)
		}
		if err := connection.Close(); err != nil {
			fmt.Println("failed to close webscoket connection: %s", err)
		}
	}()

	var connectionClosed bool
	var waiter sync.WaitGroup
	waiter.Add(1)

	// this is a keep-alive loop that ensures connection does not hang-up itself
	lastPongTime := time.Now()
	connection.SetPongHandler(func(msg string) error {
		lastPongTime = time.Now()
		return nil
	})
	go func() {
		for {
			if err := connection.WriteMessage(websocket.PingMessage, []byte("keepalive")); err != nil {
				fmt.Println("failed to write ping message")
				return
			}
			time.Sleep(keepalivePingTimeout / 2)
			if time.Now().Sub(lastPongTime) > keepalivePingTimeout {
				fmt.Println("failed to get response from ping, triggering disconnect now...")
				waiter.Done()
				return
			}
			fmt.Println("received response from ping successfully")
		}
	}()

	// tty >> xterm.js
	go func() {
		errorCounter := 0
		for {
			// consider the connection closed/errored out so that the socket handler
			// can be terminated - this frees up memory so the service doesn't get
			// overloaded
			if errorCounter > connectionErrorLimit {
				waiter.Done()
				break
			}
			buffer := make([]byte, maxBufferSizeBytes)
			readLength, err := tty.Read(buffer)
			if err != nil {
				fmt.Println("failed to read from tty: %s", err)
				if err := connection.WriteMessage(websocket.TextMessage, []byte("bye!\r\n")); err != nil {
					fmt.Println("failed to send termination message from tty to xterm.js: %s", err)
				}
				waiter.Done()
				return
			}
			if err := connection.WriteMessage(websocket.BinaryMessage, buffer[:readLength]); err != nil {
				fmt.Println("failed to send %v bytes from tty to xterm.js", readLength)
				errorCounter++
				continue
			}
			fmt.Println("sent message of size %v bytes from tty to xterm.js", readLength)
			errorCounter = 0
		}
	}()

	// tty << xterm.js
	go func() {
		for {
			// data processing
			messageType, data, err := connection.ReadMessage()
			if err != nil {
				if !connectionClosed {
					fmt.Println("failed to get next reader: %s", err)
				}
				return
			}
			dataLength := len(data)
			dataBuffer := bytes.Trim(data, "\x00")
			dataType, ok := WebsocketMessageType[messageType]
			if !ok {
				dataType = "unknown"
			}
			fmt.Println("received %s (type: %v) message of size %v byte(s) from xterm.js with key sequence: %v", dataType, messageType, dataLength, dataBuffer)

			// process
			if dataLength == -1 { // invalid
				fmt.Println("failed to get the correct number of bytes read, ignoring message")
				continue
			}

			// handle resizing
			if messageType == websocket.BinaryMessage {
				if dataBuffer[0] == 1 {
					ttySize := &TTYSize{}
					resizeMessage := bytes.Trim(dataBuffer[1:], " \n\r\t\x00\x01")
					if err := json.Unmarshal(resizeMessage, ttySize); err != nil {
						fmt.Println("failed to unmarshal received resize message '%s': %s", string(resizeMessage), err)
						continue
					}
					fmt.Println("resizing tty to use %v rows and %v columns...", ttySize.Rows, ttySize.Cols)
					if err := pty.Setsize(tty, &pty.Winsize{
						Rows: ttySize.Rows,
						Cols: ttySize.Cols,
					}); err != nil {
						fmt.Println("failed to resize tty, error: %s", err)
					}
					continue
				}
			}

			// write to tty
			bytesWritten, err := tty.Write(dataBuffer)
			if err != nil {
				fmt.Println(fmt.Sprintf("failed to write %v bytes to tty: %s", len(dataBuffer), err))
				continue
			}
			fmt.Println("%v bytes written to tty...", bytesWritten)
		}
	}()

	waiter.Wait()
	fmt.Println("closing connection...")
	connectionClosed = true
}

func getConnectionUpgrader(
	allowedHostnames []string,
	maxBufferSizeBytes int,
) websocket.Upgrader {
	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			requesterHostname := r.Host
			if strings.Index(requesterHostname, ":") != -1 {
				requesterHostname = strings.Split(requesterHostname, ":")[0]
			}
			for _, allowedHostname := range allowedHostnames {
				if requesterHostname == allowedHostname {
					return true
				}
			}
			fmt.Println("failed to find '%s' in the list of allowed hostnames ('%s')", requesterHostname)
			return false
		},
		HandshakeTimeout: 0,
		ReadBufferSize:   maxBufferSizeBytes,
		WriteBufferSize:  maxBufferSizeBytes,
	}
}
