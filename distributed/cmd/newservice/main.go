package main

import (
	"context"
	"distributed/registry"
	"distributed/service"
	"fmt"
	stlog "log"
	"distributed/new"
)

func main() {
	host := "localhost"
	port := "7000"
	serviceAddress := fmt.Sprintf("http://%s:%s", host, port)

	reg := registry.Registration{
		ServiceName: registry.NewService,
		ServiceURL:  serviceAddress,
		RequiredServices: []registry.ServiceName{registry.LogService},
		ServiceUpdateURL:  serviceAddress + "/services",
		HeartbeatURL:     serviceAddress + "/heartbeat",
	}

	ctx, err := service.Start(
		context.Background(),
		host,
		port,
		reg,
		new.RegisterHandlers,
	)

	if err != nil {
		stlog.Fatalln(err)
	}

	<-ctx.Done()
	fmt.Println("Shutting down new service")
}