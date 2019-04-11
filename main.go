package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/mia0x75/copycat/agent"
	"github.com/mia0x75/copycat/binlog"
	"github.com/mia0x75/copycat/g"
	"github.com/mia0x75/copycat/services"
)

var (
	//if debug is true, print stack log
	versionCmd = flag.Bool("version", false, "copycat version")                     //
	vCmd       = flag.Bool("v", false, "copycat version")                           //
	helpCmd    = flag.Bool("help", false, "help")                                   //
	hCmd       = flag.Bool("h", false, "help")                                      //
	daemonCmd  = flag.Bool("daemon", false, "-daemon or -d, run as daemon process") //
	dCmd       = flag.Bool("d", false, "-daemon or -d, run as daemon process")      //
)

func main() {
	flag.Parse()
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("%+v", err)
		}
	}()
	if *helpCmd || *hCmd {
		g.Usage()
		os.Exit(0)
	}
	fmt.Print(g.Banner)

	fmt.Printf("%-11s: %s\n%-11s: %s\n%-11s: %s\n%-11s: %s\n%-11s: %s\n%-11s: %s\n",
		"Version", g.Version,
		"Git commit", g.Git,
		"Compile", g.Compile,
		"Distro", g.Distro,
		"Kernel", g.Kernel,
		"Branch", g.Branch,
	)
	if *versionCmd || *vCmd {
		os.Exit(0)
	}

	g.ParseConfig("")
	// app init
	g.Init()
	ctx := g.NewContext()

	tcpService := services.NewTCPService(ctx)

	// agent代理，用于实现集群
	agentServer := agent.NewAgentServer(
		ctx,
		agent.OnEvent(tcpService.SendAll),
		agent.OnRaw(tcpService.SendRaw),
	)

	// 核心binlog服务
	blog := binlog.NewBinlog(
		ctx,
		// pos改变的时候，通过agent server同步给所有的客户端
		binlog.PosChange(func(data []byte) {
			packData := services.Pack(agent.CMD_POS, data)
			agentServer.Sync(packData)
		}),
		// 将所有的事件同步给所有的客户端
		binlog.OnEvent(func(table string, data []byte) {
			packData := services.Pack(agent.CMD_EVENT, data)
			agentServer.Sync(packData)
		}),
	)

	// 注册服务
	blog.RegisterService(tcpService)
	// 开始binlog进程
	blog.Start()

	// set agent receive pos callback
	// 延迟依赖绑定
	// agent与binlog相互依赖
	// agent收到leader的pos改变同步信息时，回调到SaveBinlogPosition
	// agent选leader成功回调到OnLeader上，是为了停止和开启服务，只有leader在工作
	agent.OnPos(blog.SaveBinlogPosition)(agentServer)
	agent.OnLeader(blog.OnLeader)(agentServer)

	// 启动agent进程
	agentServer.Start()
	defer agentServer.Close()

	if g.Config().Admin.Enabled {
		mux := http.NewServeMux()
		mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, agentServer.ShowMembers())
		})
		mux.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "reload")
		})
		mux.HandleFunc("/stop", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "stop")
		})
		mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "start")
		})
		go http.ListenAndServe(g.Config().Admin.Listen, mux)
	}

	// wait exit
	select {
	case <-ctx.Done():
	}

	ctx.Cancel()
	blog.Close()
	fmt.Println("service exit...")
}
