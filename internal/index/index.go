package index

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"html"
	"os"
	"sync"
)

var content []byte
var once sync.Once

func Handler(ctx iris.Context) {
	once.Do(setup)

	ctx.StatusCode(200)
	ctx.ContentType("text/html; charset=UTF-8")
	ctx.Write(content)
}

func setup() {
	title, ok := os.LookupEnv("TTYPUB_TITLE")
	if !ok {
		title = "tty.pub"
	}

	content = []byte(fmt.Sprintf(template, html.EscapeString(title)))
}

const template = `<!DOCTYPE html>
<html>
  <head>
    <title>%s</title>
  </head>
  <body>
  </body>
</html>
`
