package ws

import (
	"bytes"
	"fmt"
	. "github.com/Al2Klimov/tty.pub/server/internal"
	"github.com/creack/pty"
	ws "github.com/gorilla/websocket"
	"github.com/kataras/iris/v12"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type wsio struct {
	conn *ws.Conn
	rbuf bytes.Buffer
}

var _ io.Reader = (*wsio)(nil)
var _ io.Writer = (*wsio)(nil)

func (w *wsio) Read(p []byte) (n int, err error) {
	for w.rbuf.Len() < 1 {
		_, data, errRM := w.conn.ReadMessage()
		if errRM != nil {
			return 0, errRM
		}

		w.rbuf.Write(data)
	}

	return w.rbuf.Read(p)
}

func (w *wsio) Write(p []byte) (n int, err error) {
	if err = w.conn.WriteMessage(ws.BinaryMessage, p); err == nil {
		n = len(p)
	}
	return
}

const noDocker = "Couldn't start Docker CLI"

var image string
var once sync.Once

func Handler(ctx iris.Context) {
	u := ws.Upgrader{EnableCompression: true}
	conn, errUg := u.Upgrade(ctx.ResponseWriter(), ctx.Request(), nil)

	if errUg == nil {
		go handleWs(conn)
	}
}

func handleWs(conn *ws.Conn) {
	defer conn.Close()

	once.Do(setup)

	client := wsio{conn: conn}

	{
		cmd := exec.Command("docker", "pull", image)
		ptty, errPS := pty.Start(cmd)

		if errPS != nil {
			log.WithFields(log.Fields{"error": LoggableError{errPS}}).Error(noDocker)
			fmt.Fprintln(&client, noDocker)
			return
		}

		defer ptty.Close()

		if _, errCp := io.Copy(&client, ptty); errCp != nil {
			if pe, ok := errCp.(*os.PathError); !(ok && pe.Err == syscall.EIO) {
				log.WithFields(log.Fields{"error": LoggableError{errCp}}).Debug("I/O error")

				if errWt := cmd.Wait(); errWt != nil {
					log.WithFields(log.Fields{
						"image": image, "error": LoggableError{errWt},
					}).Warn("Couldn't pull image")
				}

				return
			}
		}

		if errWt := cmd.Wait(); errWt != nil {
			log.WithFields(log.Fields{"image": image, "error": LoggableError{errWt}}).Warn("Couldn't pull image")
			return
		}
	}

	// TODO
}

func setup() {
	img, ok := os.LookupEnv("TTYPUB_IMAGE")
	if !ok {
		img = "alpine"
	}

	image = img

	if errSE := os.Setenv("TERM", "xterm-256color"); errSE != nil {
		log.WithFields(log.Fields{"error": LoggableError{errSE}}).Error("Couldn't set $TERM")
	}
}
