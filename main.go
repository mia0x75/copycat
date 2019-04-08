package main

import (
	"flag"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/agent"
	"github.com/mia0x75/copycat/binlog"
	"github.com/mia0x75/copycat/control"
	"github.com/mia0x75/copycat/g"
	"github.com/mia0x75/copycat/services"
)

var (
	//if debug is true, print stack log
	debugCmd   = flag.Bool("debug", false, "enable debug, default disable")         //
	versionCmd = flag.Bool("version", false, "copycat version")                     //
	vCmd       = flag.Bool("v", false, "copycat version")                           //
	stopCmd    = flag.Bool("stop", false, "stop service")                           //
	reloadCmd  = flag.Bool("reload", false, "reload service")                       //
	helpCmd    = flag.Bool("help", false, "help")                                   //
	hCmd       = flag.Bool("h", false, "help")                                      //
	statusCmd  = flag.Bool("status", false, "show status")                          //
	daemonCmd  = flag.Bool("daemon", false, "-daemon or -d, run as daemon process") //
	dCmd       = flag.Bool("d", false, "-daemon or -d, run as daemon process")      //
)

func runCmd(ctx *g.Context) bool {
	if *versionCmd || *vCmd || *stopCmd || *reloadCmd || *helpCmd || *hCmd || *statusCmd {
		// Show version info
		if *versionCmd || *vCmd {
			fmt.Print(g.Banner)

			fmt.Printf("%-11s: %s\n%-11s: %s\n%-11s: %s\n%-11s: %s\n%-11s: %s\n%-11s: %s\n",
				"Version", g.Version,
				"Git commit", g.Git,
				"Compile", g.Compile,
				"Distro", g.Distro,
				"Kernel", g.Kernel,
				"Branch", g.Branch,
			)
			return true
		}
		// Show usage
		if *helpCmd || *hCmd {
			g.Usage()
			return true
		}
		cli := control.NewClient(ctx)
		defer cli.Close()
		// Stop service
		if *stopCmd {
			cli.Stop()
			return true
		}
		// Reload configuration
		if *reloadCmd {
			// TODO:
			// cli.Reload(*reloadCmd)
			return true
		}
		// Show service running status
		if *statusCmd {
			cli.ShowMembers()
			return true
		}
	}
	return false
}

func main() {
	flag.Parse()
	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("%+v", err)
		}
	}()

	g.ParseConfig("")
	// app init
	g.Init()
	// clear some resource after exit
	defer g.Release()
	ctx := g.NewContext()
	// if use cmd params
	if runCmd(ctx) {
		return
	}

	// return true is parent process
	if g.DaemonProcess(*daemonCmd || *dCmd) {
		return
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

	httpService := services.NewHttpService(ctx)
	tcpService := services.NewTcpService(ctx)

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
	blog.RegisterService(httpService)
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

	// 热更新reload支持
	var reload = func(name string) {
		if name == "all" {
			tcpService.Reload()
			httpService.Reload()
		} else {
			switch name {
			case httpService.Name():
				httpService.Reload()
			case tcpService.Name():
				tcpService.Reload()
			default:
				log.Errorf("[E] unknown service: %v", name)
			}
		}
	}

	// stop、reload、members ... support
	// 本地控制命令支持
	ctl := control.NewControl(
		ctx,
		control.ShowMember(agentServer.ShowMembers),
		control.Reload(reload),
		control.Stop(ctx.Stop),
	)
	ctl.Start()
	defer ctl.Close()

	// wait exit
	select {
	case <-ctx.Done():
	}

	ctx.Cancel()
	blog.Close()
	fmt.Println("service exit...")
}
