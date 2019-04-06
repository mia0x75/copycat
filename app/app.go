package app

import (
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/sevlyar/go-daemon"
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/nova/file"
	"github.com/mia0x75/nova/ip"
	mlog "github.com/mia0x75/nova/log"
	"github.com/mia0x75/nova/path"
	wstring "github.com/mia0x75/nova/string"
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
	// set log context hook
	log.SetLevel(log.DebugLevel)

	// write pid file
	data := []byte(fmt.Sprintf("%d", os.Getpid()))
	ioutil.WriteFile(PID_FILE, data, 0644)
	// get app config
	appConfig, _ := GetAppConfig()
	log.SetLevel(log.Level(appConfig.LogLevel))
	// run pprof
	go func() {
		//http://localhost:6060/debug/pprof/  内存性能分析工具
		//go tool pprof logDemo.exe --text a.prof
		//go tool pprof your-executable-name profile-filename
		//go tool pprof your-executable-name http://localhost:6060/debug/pprof/heap
		//go tool pprof nova http://localhost:6060/debug/pprof/heap
		//https://lrita.github.io/2017/05/26/golang-memory-pprof/
		//然后执行 text
		//go tool pprof -alloc_space http://127.0.0.1:6060/debug/pprof/heap
		//top20 -cum

		//下载文件 http://localhost:6060/debug/pprof/profile
		//分析 go tool pprof -web /Users/yuyi/Downloads/profile
		if appConfig.PprofListen != "" {
			http.ListenAndServe(appConfig.PprofListen, nil)
		}
	}()
	// set timezone
	time.LoadLocation(appConfig.TimeZone)
	if DEBUG {
		// set log context hook
		log.AddHook(mlog.ContextHook{})
	}
	// set cpu num
	runtime.GOMAXPROCS(runtime.NumCPU()) //指定cpu为多核运行 旧版本兼容
}

func Release() {
	// delete pid when exit
	file.Delete(PID_FILE)
	if ctx != nil {
		// release process context when exit
		ctx.Release()
	}
}

// show usage
func Usage() {
	fmt.Println("nova                                   : start service")
	fmt.Println("nova -h|-help                          : show this message")
	fmt.Println("nova -v|-version                       : show version info")
	fmt.Println("nova -stop                             : stop service")
	fmt.Println("nova -reload                           : reload")
	fmt.Println("nova -status                           : show status")
	fmt.Println("nova -d|-daemon                        : run as daemon process")
}

// get unique key, param if file path
// if file does not exists, try to create it, and write a unique key
// return the unique key
// if exists, read file and return it
func GetKey(sessionFile string) string {
	log.Debugf("key file: %s", sessionFile)
	if file.Exists(sessionFile) {
		data := file.Read(sessionFile)
		if data != "" {
			return data
		}
	}
	//write a new key
	key := fmt.Sprintf("%d-%s", time.Now().Unix(), wstring.RandString(64))
	dir := path.GetParent(sessionFile)
	path.Mkdir(dir)
	n := file.Write(sessionFile, key, false)
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

func getHttpConfig() (*HttpConfig, error) {
	var config HttpConfig
	configFile := HTTP_CONFIG_FILE
	if !file.Exists(configFile) {
		log.Warnf("config file %s does not exists", configFile)
		return nil, ErrorFileNotFound
	}
	if _, err := toml.DecodeFile(configFile, &config); err != nil {
		log.Println(err)
		return nil, ErrorFileParse
	}
	if config.TimeTick <= 0 {
		config.TimeTick = 1
	}
	return &config, nil
}

func getTcpConfig() (*TcpConfig, error) {
	configFile := TCP_CONFIG_FILE
	var err error
	if !file.Exists(configFile) {
		log.Warnf("config %s does not exists", configFile)
		return nil, ErrorFileNotFound
	}
	var tcpConfig TcpConfig
	if _, err = toml.DecodeFile(configFile, &tcpConfig); err != nil {
		log.Println(err)
		return nil, ErrorFileParse
	}
	if tcpConfig.ServiceIp == "" {
		tcpConfig.ServiceIp, err = ip.Local()
		if err != nil {
			log.Panicf("can not get local ip, please set service ip(service_ip) in file %s", configFile)
		}
	}
	if tcpConfig.ServiceIp == "" {
		log.Panicf("service ip can not be empty (config file: %s)", configFile)
	}
	if tcpConfig.Port <= 0 {
		log.Panicf("service port can not be 0 (config file: %s)", configFile)
	}
	return &tcpConfig, nil
}

func getMysqlConfig() (*MysqlConfig, error) {
	var appConfig MysqlConfig
	configFile := DB_CONFIG_FILE
	if !file.Exists(configFile) {
		log.Errorf("config file %s not found", configFile)
		return nil, ErrorFileNotFound
	}
	if _, err := toml.DecodeFile(configFile, &appConfig); err != nil {
		log.Println(err)
		return nil, ErrorFileParse
	}
	return &appConfig, nil
}

func getClusterConfig() (*ClusterConfig, error) {
	var config ClusterConfig
	configFile := CLUSTER_CONFIG_FILE
	if !file.Exists(configFile) {
		log.Errorf("config file not found: %s", configFile)
		return nil, ErrorFileNotFound
	}
	if _, err := toml.DecodeFile(configFile, &config); err != nil {
		log.Println(err)
		return nil, ErrorFileParse
	}
	return &config, nil
}
