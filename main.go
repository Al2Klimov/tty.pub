package main

import (
	"context"
	"github.com/Al2Klimov/tty.pub/internal/index"
	"github.com/kataras/golog"
	"github.com/kataras/iris/v12"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var onTerm struct {
	sync.Mutex

	toDo []func()
}

func main() {
	initLogging()
	go wait4term()

	app := iris.Default()

	app.Get("/", index.Handler)

	onTerm.Lock()
	onTerm.toDo = append(onTerm.toDo, func() {
		_ = app.Shutdown(context.Background())
	})
	onTerm.Unlock()

	_ = app.Run(iris.Addr("[::]:8080"), iris.WithoutStartupLog, iris.WithoutInterruptHandler)
}

func initLogging() {
	if !terminal.IsTerminal(int(os.Stdout.Fd())) {
		log.SetFormatter(&log.JSONFormatter{})
	}

	log.SetLevel(log.TraceLevel)
	log.SetOutput(os.Stdout)

	golog.InstallStd(log.StandardLogger())
	golog.SetLevel("debug")
}

func wait4term() {
	ch := make(chan os.Signal, 1)

	{
		signals := [2]os.Signal{syscall.SIGTERM, syscall.SIGINT}
		signal.Notify(ch, signals[:]...)
		log.WithFields(log.Fields{"signals": signals}).Trace("Listening for signals")
	}

	log.WithFields(log.Fields{"signal": <-ch}).Info("Terminating")

	onTerm.Lock()
	for _, f := range onTerm.toDo {
		f()
	}

	os.Exit(0)
}
