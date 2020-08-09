package ws

import (
	"github.com/gorilla/websocket"
	"github.com/kataras/iris/v12"
)

func Handler(ctx iris.Context) {
	u := websocket.Upgrader{EnableCompression: true}
	conn, errUg := u.Upgrade(ctx.ResponseWriter(), ctx.Request(), nil)

	if errUg == nil {
		go handleWs(conn)
	}
}

func handleWs(conn *websocket.Conn) {
	defer conn.Close()

	// TODO
}
