package main

import (
	"context"
	"distributed/grades"
	"distributed/log"
	"distributed/registry"
	"distributed/service"
	"fmt"
	stlog "log"
)

// 业务服务的启动程序

func main(){
	host, port := "localhost", "6000"
	serviceAddress := fmt.Sprintf("http://%v:%v", host, port)
	//(ctx context.Context, host, port string, reg registry.Registration,registerHandlersFunc func())

	reg := registry.Registration {
		ServiceName: registry.GradingService,
		ServiceURL: serviceAddress,
		RequiredServices: []registry.ServiceName{registry.LogService},
		ServiceUpdateURL: serviceAddress + "/services",
		HeartbeatURL:     serviceAddress + "/heartbeat",
	}

	ctx, err := service.Start(context.Background(),host,port,reg,grades.RegisterHandlers)
	if err != nil{
		stlog.Fatal(err)
	}

	if logProvider, err := registry.GetProvider(registry.LogService); err == nil {
		fmt.Printf("Logging service found at %s\n", logProvider)
		log.SetClientLogger(logProvider, reg.ServiceName)
	}
	
	// 接收服务关闭信号
	<-ctx.Done()
	fmt.Println("Shutting down grading service")
}