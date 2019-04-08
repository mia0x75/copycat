package g

import (
	"fmt"
	"io/ioutil"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strings"
	"time"

	daemon "github.com/sevlyar/go-daemon"
	log "github.com/sirupsen/logrus"
	"github.com/toolkits/file"

	"github.com/mia0x75/copycat/utils"
	"github.com/mia0x75/copycat/utils/path"
)

var ctx *daemon.Context = nil

const (
	VERSION = "1.0.0"
)

// app init
// config path parse
// cache path parse
// log path parse
// get app config
// check app is running, if pid file exists, app is running
// write pid file
// start pprof
// set logger
func Init() {
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat:  "2006-01-02 15:04:05",
		ForceColors:      true,
		QuoteEmptyFields: true,
		FullTimestamp:    true,
	})
	// write pid file
	data := []byte(fmt.Sprintf("%d", os.Getpid()))
	ioutil.WriteFile(PID_FILE, data, 0644)
	if level, err := log.ParseLevel(Config().Log.Level); err == nil {
		log.SetLevel(level)
	}
	// run pprof
	go func() {
		//http://localhost:6060/debug/pprof/  内存性能分析工具
		//go tool pprof logDemo.exe --text a.prof
		//go tool pprof your-executable-name profile-filename
		//go tool pprof your-executable-name http://localhost:6060/debug/pprof/heap
		//go tool pprof copycat http://localhost:6060/debug/pprof/heap
		//https://lrita.github.io/2017/05/26/golang-memory-pprof/
		//然后执行 text
		//go tool pprof -alloc_space http://127.0.0.1:6060/debug/pprof/heap
		//top20 -cum

		//下载文件 http://localhost:6060/debug/pprof/profile
		//分析 go tool pprof -web /Users/yuyi/Downloads/profile
		// if appConfig.PprofListen != "" {
		// 	http.ListenAndServe(appConfig.PprofListen, nil)
		// }
	}()
	// set timezone
	time.LoadLocation(Config().TimeZone)
	// set cpu num
	runtime.GOMAXPROCS(runtime.NumCPU()) //指定cpu为多核运行 旧版本兼容
}

func Release() {
	// delete pid when exit
	file.Remove(PID_FILE)
	if ctx != nil {
		// release process context when exit
		ctx.Release()
	}
}

// show usage
func Usage() {
	fmt.Println("copycat                                   : start service")
	fmt.Println("copycat -h|-help                          : show this message")
	fmt.Println("copycat -v|-version                       : show version info")
	fmt.Println("copycat -stop                             : stop service")
	fmt.Println("copycat -reload                           : reload")
	fmt.Println("copycat -status                           : show status")
	fmt.Println("copycat -d|-daemon                        : run as daemon process")
}

// get unique key, param if file path
// if file does not exists, try to create it, and write a unique key
// return the unique key
// if exists, read file and return it
func GetKey(sessionFile string) string {
	if file.IsExist(sessionFile) {
		data, _ := file.ToTrimString(sessionFile)
		if data != "" {
			return data
		}
	}
	//write a new key
	key := fmt.Sprintf("%d-%s", time.Now().Unix(), utils.RandString(64))
	dir := path.GetParent(sessionFile)
	path.Mkdir(dir) // TODO:
	n, _ := file.WriteString(sessionFile, key)
	if n != len(key) {
		return ""
	}
	return key
}

// run as daemon process
func DaemonProcess(d bool) bool {
	if d {
		exeFile := strings.Replace(os.Args[0], "\\", "/", -1)
		fileName := exeFile
		lastIndex := strings.LastIndex(exeFile, "/")
		if lastIndex > -1 {
			fileName = exeFile[lastIndex+1:]
		}
		cmd := []string{path.CurrentPath + "/" + fileName, " -daemon"}
		ctx = &daemon.Context{
			PidFileName: PID_FILE,
			PidFilePerm: 0644,
			LogFileName: LOG_FILE,
			LogFilePerm: 0640,
			WorkDir:     path.CurrentPath,
			Umask:       027,
			Args:        cmd,
		}
		d, err := ctx.Reborn()
		if err != nil {
			log.Fatal("Unable to run: ", err)
		}
		if d != nil {
			return true
		}
		return false
	}
	return false
}
