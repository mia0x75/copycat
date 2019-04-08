package control

import (
	"fmt"
	"net"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/g"
	"github.com/mia0x75/copycat/services"
)

type control struct {
	conn *net.TCPConn
}

func NewClient(ctx *g.Context) *control {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", ctx.Config.Control.Listen)
	if err != nil {
		log.Panicf("[P] start control with error: %+v", err)
	}
	con := &control{}
	con.conn, err = net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Panicf("[P] start control with error: %+v", err)
	}
	return con
}

func (con *control) Close() {
	con.conn.Close()
}

// -stop
func (con *control) Stop() {
	data := services.Pack(CMD_STOP, []byte(""))
	con.conn.Write(data)
	var buf = make([]byte, 1024)
	con.conn.SetReadDeadline(time.Now().Add(time.Second * 3))
	con.conn.Read(buf)
	fmt.Println(string(buf))
}

// Reload 热更新
func (con *control) Reload(serviceName string) {
	data := services.Pack(CMD_RELOAD, []byte(serviceName))
	con.conn.Write(data)
}

// -members
func (con *control) ShowMembers() {
	data := services.Pack(CMD_SHOW_MEMBERS, []byte(""))
	con.conn.Write(data)
	var buf = make([]byte, 40960)
	con.conn.SetReadDeadline(time.Now().Add(time.Second * 30))
	con.conn.Read(buf)
	fmt.Println(string(buf))
}
