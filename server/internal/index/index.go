// SPDX-License-Identifier: AGPL-3.0-or-later

package index

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"html"
	"os"
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

	ctx.StatusCode(200)
	ctx.ContentType("text/html; charset=UTF-8")
	ctx.SetLastModified(modtime)
	ctx.Write(content)
}

func setup() {
	title, ok := os.LookupEnv("TTYPUB_TITLE")
	if !ok {
		title = "tty.pub"
	}

	content = []byte(fmt.Sprintf(template, html.EscapeString(title)))
	modtime = time.Now()
}

const template = `<!DOCTYPE html>
<html>
  <head>
    <title>%s</title>
  </head>
  <body>
    <div id="placeholder">&#x25B6;</div>
    <div id="terminal"></div>
    <p>Powered by <a href="https://github.com/Al2Klimov/tty.pub">tty.pub</a></p>
    <link rel="stylesheet" href="xterm.css">
    <link rel="stylesheet" href="style.css">
    <script src="xterm.js"></script>
    <script src="main.js"></script>
  </body>
</html>
`
