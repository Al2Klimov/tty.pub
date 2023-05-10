// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"context"
	source_repos "github.com/Al2Klimov/go-gen-source-repos"
	. "github.com/Al2Klimov/tty.pub/server/internal"
	"github.com/Al2Klimov/tty.pub/server/internal/favicon"
	"github.com/Al2Klimov/tty.pub/server/internal/index"
	"github.com/Al2Klimov/tty.pub/server/internal/ws"
	"github.com/google/uuid"
	"github.com/kataras/golog"
	"github.com/kataras/iris/v12"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	hardening()
	uuid.EnableRandPool()
	initLogging()
	go wait4term()

	log.WithFields(log.Fields{"projects": append(source_repos.GetLinks(), "https://github.com/xtermjs/xterm.js")}).Info(
		"For the terms of use, the source code and the authors see the projects this program is assembled from",
	)

	app := iris.Default()

	app.Get("/", index.Handler)
	app.Get("/favicon.ico", favicon.Handler)
	app.Get("/v1", ws.Handler)
	app.HandleDir("/", "./www", iris.DirOptions{Compress: true})

	OnTerm.Lock()
	OnTerm.ToDo = append(OnTerm.ToDo, func() {
		_ = app.Shutdown(context.Background())
	})
	OnTerm.Unlock()

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

	close(OnTerm.Closed)
	OnTerm.Lock()

	for _, f := range OnTerm.ToDo {
		f()
	}

	os.Exit(0)
}
