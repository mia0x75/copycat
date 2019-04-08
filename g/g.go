package g

import (
	"runtime"
)

const (
	Version = "0.1.127"
	Git     = "2019-04-08 13:31:13 +0800 8dfdd77"
	Compile = "2019-04-08 14:19:52 +0800 by go version go1.12.2 darwin/amd64"
	Branch  = "master"
	Distro  = "Unknown"
	Kernel  = "18.2.0"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}
