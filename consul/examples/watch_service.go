package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/consul"
)

func main() {
	config := api.DefaultConfig()
	config.Address = "127.0.0.1:8500"

	client, _ := api.NewClient(config)
	serviceName := "service-test2"

	watch := consul.NewWatchService(client.Health(), serviceName)
	go watch.Watch(func(event int, member *consul.ServiceMember) {
		switch event {
		case consul.EventAdd:
			log.Infof("[I] %v add service: %+v", time.Now().Unix(), member)
		case consul.EventDelete:
			log.Infof("[I] %v delete service: %+v", time.Now().Unix(), member)
		case consul.EventStatusChange:
			log.Infof("[I] %v offline service: %+v", time.Now().Unix(), member)
		}
	})

	time.Sleep(time.Second * 3)

	sev := consul.NewService(client.Agent(), serviceName, "127.0.0.1", 7770)
	log.Infof("[I] %v register", time.Now().Unix())
	sev.Register()
	defer sev.Deregister()
	a := time.After(time.Second * 35)
	go func() {
		select {
		case <-a:
			sev.Deregister()
		}
	}()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		os.Kill,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	<-sc
}
