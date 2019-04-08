package g

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
	PidFile    string
	cancelChan chan struct{}
	PosChan    chan string
	Config     *GlobalConfig
}

// NewContext new app context
func NewContext() *Context {
	ctx := &Context{
		cancelChan: make(chan struct{}),
		Config:     Config(),
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

func (ctx *Context) Reload() {
	ctx.Config = Reload()
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
	log.Warnf("[W] get exit signal, service will exit later")
	ctx.cancelChan <- struct{}{}
}
