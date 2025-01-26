package main

// 日志服务的可运行程序

import (
	"context"
	"distributed/log"
	"distributed/registry"
	"distributed/service"
	"fmt"
	stlog "log"
)

func main(){
	// 创建新的日志记录器、指定输出位置
	log.Run("./distributed.log")
	// 指定 服务名称，服务监听ip端口，日志服务处理程序
	host, port := "localhost","4000"
	serviceAddress := fmt.Sprintf("http://%s:%s", host, port)

	r := registry.Registration{
		ServiceName: registry.LogService,
		ServiceURL: serviceAddress,
		RequiredServices: make([]registry.ServiceName, 0),
		ServiceUpdateURL: serviceAddress + "/services",
		HeartbeatURL:     serviceAddress + "/heartbeat",
	}

	ctx, err := service.Start(
		context.Background(),
		host,
		port,
		r,
		log.RegisterHandlers,
	)

	// 如果日志服务启动失败，记录失败原因
	if err != nil {
		stlog.Fatalln(err)
	}

	// 使用 <-ctx.Done() 阻塞等待服务结束，当服务结束时，打印退出
	<- ctx.Done()
	fmt.Println("Shutting down log service")
}