// SPDX-License-Identifier: AGPL-3.0-or-later

package ws

import (
	"bytes"
	"encoding/json"
	"fmt"
	. "github.com/Al2Klimov/tty.pub/server/internal"
	"github.com/creack/pty"
	"github.com/google/uuid"
	ws "github.com/gorilla/websocket"
	"github.com/kataras/iris/v12"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
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

type lastSeenReader struct {
	reader   io.Reader
	lastSeen int64
}

var _ io.Reader = (*lastSeenReader)(nil)

func (lsr *lastSeenReader) Read(p []byte) (n int, err error) {
	n, err = lsr.reader.Read(p)
	if n > 0 {
		atomic.StoreInt64(&lsr.lastSeen, time.Now().Unix())
	}
	return
}

const noDocker = "Couldn't start Docker CLI"
const maxInt = int(^uint(0) >> 1)

var image string
var dockerRun []string
var once sync.Once

var sessions struct {
	sync.Mutex

	sessions map[*int64]chan<- struct{}
	limit    int
}

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

	uid, errNR := uuid.NewRandom()
	if errNR != nil {
		log.WithFields(log.Fields{"error": LoggableError{errNR}}).Error("Couldn't generate UUIDv4")
		return
	}

	client := &wsio{conn: conn}
	lsr := &lastSeenReader{client, 0}
	kick := make(chan struct{}, 1)

	sessions.Lock()

	for len(sessions.sessions) >= sessions.limit {
		var longestInactive struct {
			pos      *int64
			lastSeen int64
		}

		for lastSeenP := range sessions.sessions {
			if lastSeen := atomic.LoadInt64(lastSeenP); lastSeen != 0 {
				if longestInactive.lastSeen == 0 || lastSeen < longestInactive.lastSeen {
					longestInactive.pos = lastSeenP
					longestInactive.lastSeen = lastSeen
				}
			}
		}

		if longestInactive.pos != nil {
			sessions.sessions[longestInactive.pos] <- struct{}{}
			delete(sessions.sessions, longestInactive.pos)
			break
		}

		runtime.Gosched()
	}

	sessions.sessions[&lsr.lastSeen] = kick

	sessions.Unlock()

	defer release(&lsr.lastSeen)

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

	{
		cmd := exec.Command("docker", "image", "prune", "-f")
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
					log.WithFields(log.Fields{"error": LoggableError{errWt}}).Warn("Couldn't clean up images")
				}

				return
			}
		}

		if errWt := cmd.Wait(); errWt != nil {
			log.WithFields(log.Fields{"error": LoggableError{errWt}}).Warn("Couldn't clean up images")
			return
		}
	}

	var args []string
	args = append(args, dockerRun[0])
	args = append(args, "--name")
	args = append(args, uid.String())
	args = append(args, dockerRun[1:]...)

	cmd := exec.Command("docker", args...)
	ptty, errPS := pty.Start(cmd)

	if errPS != nil {
		log.WithFields(log.Fields{"error": LoggableError{errPS}}).Error(noDocker)
		fmt.Fprintln(client, noDocker)
		return
	}

	defer ptty.Close()

	{
		r := make(chan struct{}, 1)
		w := make(chan struct{}, 1)

		atomic.StoreInt64(&lsr.lastSeen, time.Now().Unix())
		go cp(lsr, ptty, r)
		go cp(ptty, client, w)

		select {
		case <-r:
		case <-w:
		case <-kick:
			defer kickMsg(client, w)
		case <-OnTerm.Closed:
		}
	}

	{
		cmd := exec.Command("docker", "kill", uid.String())
		if errSt := cmd.Start(); errSt == nil {
			cmd.Wait()
		} else {
			log.WithFields(log.Fields{"error": LoggableError{errSt}}).Error(noDocker)
		}
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
	dockerRun = []string{"run", "--rm", "-it"}

	if runOpts, ok := os.LookupEnv("TTYPUB_RUNOPTS"); ok {
		var opts []string
		if errUJ := json.Unmarshal([]byte(runOpts), &opts); errUJ == nil {
			dockerRun = append(dockerRun, opts...)
		} else {
			log.WithFields(log.Fields{"error": LoggableError{errUJ}}).Error("Bad $TTYPUB_RUNOPTS")
		}
	}

	dockerRun = append(dockerRun, image)

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

	if limit, ok := os.LookupEnv("TTYPUB_SESSIONS"); ok {
		if limit, errPU := strconv.ParseUint(limit, 10, 64); errPU == nil {
			sessions.limit = int(limit)
		} else {
			log.WithFields(log.Fields{"error": LoggableError{errPU}}).Error("Bad $TTYPUB_SESSIONS")
			sessions.limit = maxInt
		}
	} else {
		sessions.limit = maxInt
	}

	sessions.sessions = map[*int64]chan<- struct{}{}
}

func release(session *int64) {
	sessions.Lock()
	delete(sessions.sessions, session)
	sessions.Unlock()
}

func cp(from io.Reader, to io.Writer, done chan<- struct{}) {
	if _, errCp := io.Copy(to, from); errCp != nil {
		if pe, ok := errCp.(*os.PathError); !(ok && pe.Err == syscall.EIO) {
			log.WithFields(log.Fields{"error": LoggableError{errCp}}).Debug("I/O error")
		}
	}

	done <- struct{}{}
}

func kickMsg(to io.Writer, once <-chan struct{}) {
	<-once
	fmt.Fprint(to, "\x1b[31m\x1b[1mI'm sorry Dave, I'm afraid you've been the longest inactive user.\x1b[0m\r\n")
}
