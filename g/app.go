package g

import (
	"fmt"
	"io/ioutil"
	_ "net/http/pprof"
	"os"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/toolkits/file"

	"github.com/mia0x75/copycat/utils"
)

// Init app init
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

// Usage show usage
func Usage() {
	fmt.Println("copycat                                   : start service")
	fmt.Println("copycat -h|-help                          : show this message")
	fmt.Println("copycat -v|-version                       : show version info")
}

// GetKey get unique key, param if file path
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
	n, _ := file.WriteString(sessionFile, key)
	if n != len(key) {
		return ""
	}
	return key
}
