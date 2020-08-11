package ws

import (
	"bytes"
	"encoding/json"
	"fmt"
	. "github.com/Al2Klimov/tty.pub/server/internal"
	"github.com/creack/pty"
	ws "github.com/gorilla/websocket"
	"github.com/kataras/iris/v12"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
	"strconv"
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
const maxInt = int(^uint(0) >> 1)

var semaphore chan struct{}
var image string
var dockerRun []string
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

	client := &wsio{conn: conn}

	semaphore <- struct{}{}
	defer release()

	OnTerm.RLock()
	defer OnTerm.RUnlock()

	{
		cmd := exec.Command("docker", "pull", image)
		ptty, errPS := pty.Start(cmd)

		if errPS != nil {
			log.WithFields(log.Fields{"error": LoggableError{errPS}}).Error(noDocker)
			fmt.Fprintln(client, noDocker)
			return
		}

		defer ptty.Close()

		if _, errCp := io.Copy(client, ptty); errCp != nil {
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

	cmd := exec.Command("docker", dockerRun...)
	ptty, errPS := pty.Start(cmd)

	if errPS != nil {
		log.WithFields(log.Fields{"error": LoggableError{errPS}}).Error(noDocker)
		fmt.Fprintln(client, noDocker)
		return
	}

	defer ptty.Close()

	{
		ch := make(chan struct{}, 2)

		go cp(client, ptty, ch)
		go cp(ptty, client, ch)

		select {
		case <-ch:
		case <-OnTerm.Closed:
		}
	}

	if p := cmd.Process; p != nil {
		p.Signal(syscall.SIGTERM)
	}

	if errWt := cmd.Wait(); errWt != nil {
		log.WithFields(log.Fields{"image": image, "error": LoggableError{errWt}}).Debug("Container exited")
	}
}

func setup() {
	img, ok := os.LookupEnv("TTYPUB_IMAGE")
	if !ok {
		img = "alpine"
	}

	image = img
	dockerRun = []string{"run", "--rm", "-it", image}

	if cmd, ok := os.LookupEnv("TTYPUB_CMD"); ok {
		var command []string
		if errUJ := json.Unmarshal([]byte(cmd), &command); errUJ == nil {
			dockerRun = append(dockerRun, command...)
		} else {
			log.WithFields(log.Fields{"error": LoggableError{errUJ}}).Error("Bad $TTYPUB_CMD")
		}
	}

	if errSE := os.Setenv("TERM", "xterm-256color"); errSE != nil {
		log.WithFields(log.Fields{"error": LoggableError{errSE}}).Error("Couldn't set $TERM")
	}

	if sessions, ok := os.LookupEnv("TTYPUB_SESSIONS"); ok {
		if limit, errPU := strconv.ParseUint(sessions, 10, 64); errPU == nil {
			semaphore = make(chan struct{}, limit)
		} else {
			log.WithFields(log.Fields{"error": LoggableError{errPU}}).Error("Bad $TTYPUB_SESSIONS")
			semaphore = make(chan struct{}, maxInt)
		}
	} else {
		semaphore = make(chan struct{}, maxInt)
	}
}

func release() {
	<-semaphore
}

func cp(from io.Reader, to io.Writer, done chan<- struct{}) {
	if _, errCp := io.Copy(to, from); errCp != nil {
		if pe, ok := errCp.(*os.PathError); !(ok && pe.Err == syscall.EIO) {
			log.WithFields(log.Fields{"error": LoggableError{errCp}}).Debug("I/O error")
		}
	}

	done <- struct{}{}
}
