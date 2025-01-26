package service

// 总服务:通用服务

import (
	"context"
	"distributed/registry"
	"fmt"
	"log"
	"net/http"
)

// 启动服务
func Start(ctx context.Context, host, port string, reg registry.Registration,registerHandlersFunc func())(context.Context,error) {
	
	// 每个服务的路由
	registerHandlersFunc()
	// 启动服务的http服务器
	ctx = startService(ctx, reg.ServiceName,host,port)

	// 在启动http服务器后，再请求注册服务
	err := registry.RegisterService(reg)
	if err != nil {
		return ctx, err
	}
	
	return ctx, nil
}

// 用于实际启动 HTTP 服务器
func startService(ctx context.Context, serviceName registry.ServiceName, host, port string) context.Context {

	// 可以创建一个可取消的上下文，通过传递取消信号来控制取消操作
	ctx,cancle := context.WithCancel(ctx)
	
	var server http.Server
	server.Addr = host + ":" + port
	
	// 启动 HTTP 服务器
	go func() {
		log.Println(server.ListenAndServe())

		// 关闭总服务
		err := registry.ShutdownService(fmt.Sprintf("http://%s:%s", host, port))
		if err != nil {
			log.Println(err)
		}
		// 服务器停止时调用 cancel() 取消上下文。
		// TODO
		// 取消上下文在 HTTP 服务器中的作用主要是协调资源释放和实现优雅关闭，
		// 确保服务器在停止时能够正确处理正在进行的操作，避免资源泄漏和数据不一致的问题。
		cancle()
	}()

	// 手动强制关闭
	go func(){
		fmt.Printf("%v started. Press any key to stop. \n", serviceName)
		var s string
		fmt.Scanln(&s)	
		// 关闭总服务
		err := registry.ShutdownService(fmt.Sprintf("http://%s:%s", host, port))
		if err != nil {
			log.Println(err)
		}
		server.Shutdown(ctx)
		cancle()
	}()

	return ctx
}