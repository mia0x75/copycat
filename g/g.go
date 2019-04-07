package g

import (
	"runtime"
)

const (
	Version = "0.1.121"
	Git     = "2019-04-07 16:27:23 +0800 a10dcc8"
	Compile = "2019-04-07 21:41:57 +0800 by go version go1.11.5 linux/amd64"
	Branch  = "master"
	Distro  = "Fedora release 29 (Twenty Nine)"
	Kernel  = "5.0.5-200.fc29.x86_64"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}
