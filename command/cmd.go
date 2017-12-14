package command

import (
	"github.com/mia0x75/centineld/unix"
)

func Stop() {
	client := unix.NewUnixClient()
	msg := client.Pack(unix.CMD_STOP, "")
	client.Send(msg)
	client.Close()
}

func Reload(cmd string) {
	client := unix.NewUnixClient()
	msg := client.Pack(unix.CMD_RELOAD, cmd)
	client.Send(msg)
	client.Close()
}
