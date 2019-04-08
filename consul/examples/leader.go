package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/consul"
)

func main() {
	serviceName := "service-test"
	lockKey := "test"
	address := "127.0.0.1:8500"

	leader1 := consul.NewLeader(address, lockKey, serviceName, "127.0.0.1", 7770)
	leader1.Select(func(member *consul.ServiceMember) {
		log.Infof("1=>%+v", member)
	})
	//defer leader1.Free()
	// wait a second, and start anther service
	//time.Sleep(time.Second)

	leader2 := consul.NewLeader(address, lockKey, serviceName, "127.0.0.1", 7771)
	leader2.Select(func(member *consul.ServiceMember) {
		log.Infof("2=>%+v", member)
	})

	go func() {
		for {
			l, e := leader1.Get()
			log.Infof("%v, %v", l, e)
			time.Sleep(time.Second)
		}
	}()

	//defer leader2.Free()
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
