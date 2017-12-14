package main
/*
	centineld

	configure: /etc/centineld/centineld.yml
	execute:   /usr/bin/centineld
	systemd:   /usr/lib/systemd/system/centineld.service
	log:       /var/log/centineld.log
	cache:     /run/centineld/centineld.cache
	pid:       /run/centineld/centineld.pid
	sock:      /run/centineld/centineld.sock
	rotate:    /etc/logrotate.d/centineld.cfg
*/
import (
	"github.com/mia0x75/centineld/binlog"
	"github.com/mia0x75/centineld/services"
	"github.com/mia0x75/centineld/app"

	_ "github.com/go-sql-driver/mysql"
	"runtime"
	"os"
	"os/signal"
	"syscall"
	_ "net/http/pprof"
	"net/http"
	"fmt"
	"io/ioutil"
	"strconv"
	_ "flag"
	log "github.com/sirupsen/logrus"

	_ "github.com/mia0x75/centineld/cluster"
	"github.com/mia0x75/centineld/unix"
	"github.com/mia0x75/centineld/command"
	"flag"
	"time"
	"context"
)

var (
	debug = flag.Bool("debug", false, "启用调试模式，默认为false")
	version = flag.Bool("version", false, "版本信息")
	stop = flag.Bool("stop", false, "停止服务")
	service_reload = flag.String("service-reload", "", "重新加载配置，比如重新加载http服务配置：-service-reload http")
)

const (
	VERSION = "1.0.0"
)

const banner = `
                              _                           
                       __    (_)                __      __
  _____ ____   ____   / /_  __  ____   ____    / / ____/ /
 / ___// __ \ / __ \ / __/ / / / __ \ / __ \  / / / __  / 
/ /__ / ____// / / // /_  / / / / / // ____/ / / / /_/ /  
\___/ \____//_/ /_/ \__/ /_/ /_/ /_/ \____/ /_/  \__,_/   
                                                          
`

func writePid() {
	var data_str = []byte(fmt.Sprintf("%d", os.Getpid()));
	ioutil.WriteFile("/run/centineld/centineld.pid", data_str, 0777)  //写入文件(字节数组)
}

func killPid() {
	dat, _ := ioutil.ReadFile("/run/centineld/centineld.pid")
	fmt.Print(string(dat))
	pid, _ := strconv.Atoi(string(dat))
	log.Println("给进程发送终止信号：", pid)
	//err := syscall.Kill(pid, syscall.SIGTERM)
	//log.Println(err)
}

func pprofService() {
	go func() {
		//http://localhost:6060/debug/pprof/  内存性能分析工具
		//go tool pprof logDemo.exe --text a.prof
		//go tool pprof your-executable-name profile-filename
		//go tool pprof your-executable-name http://localhost:6060/debug/pprof/heap
		//go tool pprof wing-binlog-go http://localhost:6060/debug/pprof/heap
		//https://lrita.github.io/2017/05/26/golang-memory-pprof/
		//然后执行 text
		//go tool pprof -alloc_space http://127.0.0.1:6060/debug/pprof/heap
		//top20 -cum

		//下载文件 http://localhost:6060/debug/pprof/profile
		//分析 go tool pprof -web /Users/yuyi/Downloads/profile
		log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}()
}

func init() {
	defer func() {
		if err := recover(); err != nil {
			log.Panicf("%v\n", err)
		}
		//fmt.Printf(string(debug.Stack()))
	}()
	time.LoadLocation("Local")
	log.SetFormatter(&log.TextFormatter{TimestampFormat:"2006-01-02 15:04:05",
		ForceColors:true,
		QuoteEmptyFields:true, FullTimestamp:true})
	app_config, _ := app.GetAppConfig()
	log.SetLevel(log.Level(app_config.LogLevel))//log.DebugLevel)
	writePid()
}

func main() {
	flag.Parse()
	if (*stop) {
		command.Stop()
		return
	}
	fmt.Print(banner)
	fmt.Printf("centineld version: %s\n", VERSION)
	fmt.Printf("process id: %d\n", os.Getpid())
	if (*version) {
		fmt.Println(VERSION)
		return
	}
	pprofService()
	cpu := runtime.NumCPU()
	runtime.GOMAXPROCS(cpu) //指定cpu为多核运行 旧版本兼容
	ctx, cancel := context.WithCancel(context.Background())

	tcp_service  := services.NewTcpService()
	http_service := services.NewHttpService()

	blog := binlog.NewBinlog()
	// 注册服务
	blog.BinlogHandler.RegisterService("tcp", tcp_service)
	blog.BinlogHandler.RegisterService("http", http_service)
	blog.Start(&ctx)

	server := unix.NewUnixServer()
	server.Start(blog, &cancel)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		os.Kill,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	<-sc
	// 优雅的退出程序
	cancel()
	blog.Close()
	fmt.Println("程序退出...")
}
