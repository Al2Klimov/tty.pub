// SPDX-License-Identifier: AGPL-3.0-or-later

package favicon

import (
	"bytes"
	. "github.com/Al2Klimov/tty.pub/server/internal"
	"github.com/kataras/iris/v12"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"sync"
	"time"
)

var content []byte
var modtime time.Time
var once sync.Once

func Handler(ctx iris.Context) {
	once.Do(setup)

	if modified, errMS := ctx.CheckIfModifiedSince(modtime); errMS == nil && !modified {
		ctx.WriteNotModified()
		return
	}

	ctx.SetLastModified(modtime)

	if content == nil {
		ctx.StatusCode(500)
	} else {
		ctx.StatusCode(200)
		ctx.ContentType("image/png")
		ctx.Write(content)
	}
}

func setup() {
	favicon, ok := os.LookupEnv("TTYPUB_FAVICON")
	if !ok {
		favicon = ">_"
	}

	var out, err bytes.Buffer

	cmd := exec.Command(
		"convert",
		"-background", "black", "-fill", "white",
		"-font", "Liberation-Mono", "-pointsize", "288",
		"label:"+favicon, "png:-",
	)

	cmd.Stdout = &out
	cmd.Stderr = &err

	if errRn := cmd.Run(); errRn == nil {
		content = out.Bytes()
	} else {
		log.WithFields(log.Fields{
			"error": LoggableError{errRn}, "stderr": LoggableStringer{&err},
		}).Error("Favicon generation failed")
	}

	modtime = time.Now()
}
