package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// context
type Context struct {
	// canal context
	Ctx context.Context
	// canal context func
	Cancel context.CancelFunc
	// pid file path
	PidFile     string
	cancelChan  chan struct{}
	PosChan     chan string
	HttpConfig  *HttpConfig
	TcpConfig   *TcpConfig
	MysqlConfig *MysqlConfig
	AppConfig   *AppConfig
}

// NewContext new app context
func NewContext() *Context {
	httpConfig, _ := getHttpConfig()
	tcpConfig, _ := getTcpConfig()
	mysqlConfig, _ := getMysqlConfig()
	appConfig, _ := GetAppConfig()
	ctx := &Context{
		cancelChan:  make(chan struct{}),
		HttpConfig:  httpConfig, // TODO:
		TcpConfig:   tcpConfig,
		MysqlConfig: mysqlConfig,
		AppConfig:   appConfig,
	}
	ctx.Ctx, ctx.Cancel = context.WithCancel(context.Background())
	go ctx.signalHandler()
	return ctx
}

func (ctx *Context) Stop() {
	ctx.cancelChan <- struct{}{}
}

func (ctx *Context) Done() <-chan struct{} {
	return ctx.cancelChan
}

func (ctx *Context) ReloadHttpConfig() {
	httpConfig, err := getHttpConfig()
	if err != nil {
		log.Errorf("get http config error: %v", err)
		return
	}
	ctx.HttpConfig = httpConfig
}

// ReloadTcpConfig
func (ctx *Context) ReloadTcpConfig() {
	tcpConfig, err := getTcpConfig()
	if err != nil {
		log.Errorf("get tcp config error: %v", err)
		return
	}
	ctx.TcpConfig = tcpConfig
}

// wait for control + c signal
func (ctx *Context) signalHandler() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		os.Kill,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	<-sc
	log.Warnf("get exit signal, service will exit later")
	ctx.cancelChan <- struct{}{}
}
