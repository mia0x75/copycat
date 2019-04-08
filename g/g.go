package g

import (
	"runtime"
)

const (
	Version = "0.1.136"
	Git     = "2019-04-08 14:20:46 +0800 c97f682"
	Compile = "2019-04-08 14:33:15 +0800 by go version go1.12.2 darwin/amd64"
	Branch  = "master"
	Distro  = "Unknown"
	Kernel  = "18.2.0"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}
