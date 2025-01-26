package main

import (
	"context"
	"distributed/log"
	"distributed/registry"
	"distributed/service"
	"fmt"
	stlog "log"
	"net/http"
)

func main() {
	host := "localhost"
	port := "5000"
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

	if logProvider, err := registry.GetProvider(registry.LogService); err == nil {
		fmt.Printf("Logging service found at %s\n", logProvider)
		log.SetClientLogger(logProvider, reg.ServiceName)
	} else {
		fmt.Println("Logging service not found: " + err.Error())
	}

	<-ctx.Done()
	fmt.Println("Shutting down new service")
}