package services

import (
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/nova/app"
)

type control struct {
	conn *net.TCPConn
}

func NewControl(ctx *app.Context) *control {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", fmt.Sprintf("%s:%d", ctx.TcpConfig.ServiceIp, ctx.TcpConfig.Port))
	if err != nil {
		log.Panicf("start control with error: %+v", err)
	}
	con := &control{}
	con.conn, err = net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Panicf("start control with error: %+v", err)
	}
	con.auth()
	return con
}

func (con *control) auth() {
	token := app.GetKey(app.TOKEN_FILE)
	log.Debugf("token(%d): %s", len(token), token)
	data := PackPro(FlagControl, []byte(token))
	con.conn.Write(data)
	var buf = make([]byte, 1024)
	con.conn.SetReadDeadline(time.Now().Add(time.Second * 3))
	con.conn.Read(buf)
}

func (con *control) Close() {
	con.conn.Close()
}

// -stop
func (con *control) Stop() {
	data := pack(CMD_STOP, []byte(""))
	con.conn.Write(data)
	var buf = make([]byte, 1024)
	con.conn.SetReadDeadline(time.Now().Add(time.Second * 3))
	con.conn.Read(buf)
	fmt.Println(string(buf))
}

func (con *control) Reload() {
	data := pack(CMD_RELOAD, []byte(""))
	con.conn.Write(data)
}

func (con *control) Restart() {

}

func (con *control) ShowStatus() {
	data := pack(CMD_SHOW_MEMBERS, []byte(""))
	con.conn.Write(data)
	var buf = make([]byte, 40960)
	con.conn.SetReadDeadline(time.Now().Add(time.Second * 30))
	con.conn.Read(buf)
	fmt.Println(string(buf))
}
