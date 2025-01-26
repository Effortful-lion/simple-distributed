package main

import (
	"context"
	"distributed/registry"
	"fmt"
	"log"
	"net/http"
)

// 注册服务的可运行程序


func main(){

	// 加载之前保存的注册信息
    err := registry.LoadFromFile()
    if err != nil {
        log.Fatalf("Failed to load registry from file: %v", err)
    }

	// 相当于在这里开启一个go协程，不阻塞主线程
	registry.SetupRegistryService()

	http.Handle("/services", &registry.RegistryService{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var server http.Server
	server.Addr = registry.ServerPort

	// 服务出现错误，打印到log，然后取消
	go func() {
		log.Println(server.ListenAndServe())
		cancel()
	}()

	// 手动取消服务
	go func() {
		fmt.Println("Registry service started. Press any key to stop")
		var s string
		fmt.Scanln(&s)
		server.Shutdown(ctx)
		cancel()
	}()

	<- ctx.Done()
	fmt.Println("Shutting down registry service")
}