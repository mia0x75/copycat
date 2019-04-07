package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/mac/hack"
	"github.com/mia0x75/nova/agent"
	"github.com/mia0x75/nova/app"
	"github.com/mia0x75/nova/binlog"
	"github.com/mia0x75/nova/control"
	"github.com/mia0x75/nova/g"
	"github.com/mia0x75/nova/services"
)

var (
	//if debug is true, print stack log
	debugCmd   = flag.Bool("debug", false, "enable debug, default disable")         //
	versionCmd = flag.Bool("version", false, "nova version")                        //
	vCmd       = flag.Bool("v", false, "nova version")                              //
	stopCmd    = flag.Bool("stop", false, "stop service")                           //
	reloadCmd  = flag.Bool("reload", false, "reload service")                       //
	helpCmd    = flag.Bool("help", false, "help")                                   //
	hCmd       = flag.Bool("h", false, "help")                                      //
	statusCmd  = flag.Bool("status", false, "show status")                          //
	daemonCmd  = flag.Bool("daemon", false, "-daemon or -d, run as daemon process") //
	dCmd       = flag.Bool("d", false, "-daemon or -d, run as daemon process")      //
)

const banner = "\n" +
	"ooo. .oo.    .ooooo.  oooo    ooo  .oooo.   \n" +
	"`888P\"Y88b  d88' `88b  `88.  .8'  `P  )88b  \n" +
	" 888   888  888   888   `88..8'    .oP\"888  \n" +
	" 888   888  888   888    `888'    d8(  888  \n" +
	"o888o o888o `Y8bod8P'     `8'     `Y888\"\"8o \n"

func runCmd(ctx *app.Context) bool {
	if *versionCmd || *vCmd || *stopCmd || *reloadCmd || *helpCmd || *hCmd || *statusCmd {
		// Show version info
		if *versionCmd || *vCmd {
			fmt.Print(banner)

			log.Infof("git commit: %s", g.Version)
			log.Infof("build time: %s", g.Compile)
			log.Infof("system: %s/%s", runtime.GOOS, runtime.GOARCH)
			log.Infof("version: %s", app.VERSION)
			return true
		}
		// Show usage
		if *helpCmd || *hCmd {
			app.Usage()
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
	// app init
	app.DEBUG = *debugCmd
	app.Init()
	// clear some resource after exit
	defer app.Release()
	ctx := app.NewContext()
	// if use cmd params
	if runCmd(ctx) {
		return
	}

	// return true is parent process
	if app.DaemonProcess(*daemonCmd || *dCmd) {
		return
	}

	fmt.Println(banner)

	fmt.Printf("git commit: %s\n", hack.Version)
	fmt.Printf("build time: %s\n", hack.Compile)
	fmt.Printf("system: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("process id: %d\n", os.Getpid())
	fmt.Printf("version: %s\n", app.VERSION)

	httpService := services.NewHttpService(ctx)
	tcpService := services.NewTcpService(ctx)

	agentServer := agent.NewAgentServer(
		ctx,
		agent.OnEvent(tcpService.SendAll),
		agent.OnRaw(tcpService.SendRaw),
	)

	blog := binlog.NewBinlog(
		ctx,
		binlog.PosChange(agentServer.SendPos),
		binlog.OnEvent(agentServer.SendEvent),
	)

	blog.RegisterService(tcpService)
	blog.RegisterService(httpService)
	blog.Start()

	// set agent receive pos callback
	// 延迟依赖绑定
	// agent与binlog相互依赖
	agent.OnPos(blog.SaveBinlogPosition)(agentServer)
	agent.OnLeader(blog.OnLeader)(agentServer)

	agentServer.Start()
	defer agentServer.Close()

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
				log.Errorf("unknown service: %v", name)
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
