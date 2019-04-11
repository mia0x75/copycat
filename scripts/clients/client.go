package main

import (
	"fmt"
	"os"
	"os/signal"

	log "github.com/sirupsen/logrus"

	"github.com/mia0x75/copycat/client"
)

func main() {
	//初始化debug终端输出日志支持
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat:  "2006-01-02 15:04:05",
		ForceColors:      true,
		QuoteEmptyFields: true,
		FullTimestamp:    true,
	})
	log.SetLevel(log.Level(5))

	// event callback
	// 有事件过来的时候，就会进入这个回调
	var onEvent = func(data map[string]interface{}) {
		fmt.Printf("new event: %+v", data)
	}

	// 创建一个新的客户端
	// 第一个参数为一个数组，这里可以指定多个地址，当其中一个失败的时候自动轮训下一个
	// 简单的高可用方案支持
	// 第二个参数为注册事件回调

	defaultDns := "127.0.0.1:9990"
	if len(os.Args) >= 2 {
		defaultDns = os.Args[1]
	}
	c := client.NewClient(client.SetServices([]string{defaultDns}), client.OnEventOption(onEvent))

	//或者使用consul服务发现
	//c := client.NewClient(wclient.SetConsulAddress("127.0.0.1:8500"), client.OnEventOption(onEvent))

	// 程序退出时 close 掉客户端
	defer c.Close()
	//订阅感兴趣的数据库、表变化事件
	//如果不订阅，默认对所有的变化感兴趣
	//c.Subscribe("new_yonglibao_c.*", "test.*")

	// 等待退出信号，比如control+c
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	<-signals
}
