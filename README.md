# 实现一个简单的日志服务

## 服务注册

日志服务

log/server.go

```go
package log

// 实现一个简单的日志服务

import (
	"io"
	stlog "log"
	"net/http"
	"os"
)

// 定义全局的 logger 日志记录器
var log *stlog.Logger

// 定义 fileLog类型 新类型 取代名是“=”
type fileLog string

// fileLog 实现 Write 方法：用于将日志数据写入指定的文件。
func (fl fileLog) Write (data []byte)(int, error){
	// os.Open(): 以只读模式打开；os.OpenFile(): filename，打开模式，指定文件的权限
	f, err := os.OpenFile(string(fl), os.O_CREATE | os.O_WRONLY | os.O_APPEND, 0600)
	if err != nil {
		return 0, err
	}
	// 文件资源打开后，最后关闭
	defer f.Close()
	return f.Write(data)
}

// 创建新的日志记录器、指定输出位置
func Run(dest string) {
	// new : 创建一个新的logger，参数指定：输出路径, 日志记录的固定前缀, 日志的选项：flag
	log = stlog.New(fileLog(dest),"go ",stlog.LstdFlags)
}

// 注册一个http处理程序：处理 log 路径的 POST 请求，将请求体中的消息写入日志。
func RegisterHandlers() {
	http.HandleFunc("/log",func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			msg, err := io.ReadAll(r.Body)
			if err != nil || len(msg) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			write(string(msg))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	})
}

// 写入日志的函数
func write(msg string) {
	log.Printf("%v\n",msg)
}
```

service/service.go

```go
package service

// 总服务

import (
	"context"
	"fmt"
	"log"
	"net/http"
)

// 启动服务
func Start(ctx context.Context, serviceName, host, port string, registerHandlersFunc func())(context.Context,error) {
	
	registerHandlersFunc()
	ctx = startService(ctx, serviceName,host,port)
	
	return ctx, nil
}

// 用于实际启动 HTTP 服务器
func startService(ctx context.Context, serviceName, host, port string) context.Context {

	ctx,cancle := context.WithCancel(ctx)
	
	var server http.Server
	server.Addr = host + ":" + port
	
	go func() {
		// 启动 HTTP 服务器
		log.Println(server.ListenAndServe())
		// 服务器停止时调用 cancel() 取消上下文。
		cancle()
	}()

	go func(){
		fmt.Printf("%v started. Press any key to stop. \n", serviceName)
		var s string
		fmt.Scanln(&s)	
		server.Shutdown(ctx)
		cancle()
	}()

	return ctx
}
```

cmd/logservice/main.go

```go
package main

import (
	"context"
	"distributed/log"
	"distributed/service"
	"fmt"
	stlog "log"
)

func main(){
	// 创建新的日志记录器、指定输出位置
	log.Run("./distributed.log")
	// 启动 日志服务，指定 服务名称，服务监听ip端口，日志服务处理程序
	host, port := "localhost","4000"
	ctx, err := service.Start(
		context.Background(),
		"Log Service",
		host,
		port,
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
```

服务注册服务

registry/registration.go

```go
package registry

// 服务注册


// 注册记录：服务名 + 访问地址
type Registration struct{
	ServiceName ServiceName
	ServiceURL  string
}

type ServiceName string

// 已经存在的服务：
const(
	LogService = ServiceName("LogService")
)
```

registry/server.go

```go
package registry

// 服务注册的web service

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

// 服务注册的ip和端口
const ServerPort = ":3000"
const ServicesURL = "http://localhost" + ServerPort + "/services" // 通过这个url可以查询到哪些服务？

// 服务注册中心（相当于服务注册的表：记录1--服务1，记录2--服务2...）
type registry struct {
	registrations []Registration // 已经注册的服务（可能多个线程并发访问，并且是动态变化的，所以要确保并发安全性）
	mutex         *sync.Mutex    // 互斥锁
}

// 注册方法
func (r *registry) add(reg Registration) error {
	// 加锁解锁 ，避免并发问题

	r.mutex.Lock()

	// 增加服务记录到服务表
	r.registrations = append(r.registrations, reg)

	r.mutex.Unlock()

	return nil
}

// 定义服务表结构（服务数0，互斥锁）
var reg = registry{
	registrations: make([]Registration, 0),
	mutex:         new(sync.Mutex),
}

// 定义 注册服务的实现逻辑（handler）
type RegistryService struct{}

// 实现 ServerHttp 方法
func (s RegistryService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Request received")
	switch r.Method {
	case http.MethodPost:
		dec := json.NewDecoder(r.Body)
		var r Registration
		err := dec.Decode(&r)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("Adding service: %v with URL: %s\n", r.ServiceName, r.ServiceURL)
		err = reg.add(r)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}
```

registry/client.go

```go
package registry

// 相当于一个客户端（为客户端提供函数 向注册服务（registryservice）发送请求以注册一个服务）
// 内部发送http请求，用于进行服务注册
import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// 向 registryservice 发送请求
func RegisterService(r Registration) error {

	// 创建 JSON 编码器

	// 创建一个新的缓冲区 buf ，缓冲区实现了io.Writer 和 io.Reader可以进行数据的读取和写入
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	err := enc.Encode(r)
	if err != nil {
		return nil
	}
	
	res, err := http.Post(ServicesURL,"application/json", buf)
	if err != nil {
		return err
	}

	// 如果注册失败
	if res.StatusCode != http.StatusOK {
		// 错误小写开头--惯例
		return fmt.Errorf("failed to register service. Registry service responded with code %v", res.StatusCode)
	}

	return nil
}
```

cmd\registryservice\main.go

```go
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
```

根据这个修改后的service/service.go

```go
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
	
	registerHandlersFunc()
	ctx = startService(ctx, reg.ServiceName,host,port)

	// 在启动 web server 后 再启动 注册服务
	err := registry.RegisterService(reg)
	if err != nil {
		return ctx, nil
	}
	
	return ctx, nil
}

// 用于实际启动 HTTP 服务器
func startService(ctx context.Context, serviceName registry.ServiceName, host, port string) context.Context {

	ctx,cancle := context.WithCancel(ctx)
	
	var server http.Server
	server.Addr = host + ":" + port
	
	go func() {
		// 启动 HTTP 服务器
		log.Println(server.ListenAndServe())
		// 服务器停止时调用 cancel() 取消上下文。
		cancle()
	}()

	go func(){
		fmt.Printf("%v started. Press any key to stop. \n", serviceName)
		var s string
		fmt.Scanln(&s)	
		server.Shutdown(ctx)
		cancle()
	}()

	return ctx
}
```

取消服务：registry/server.go中添加取消注册的方法

```go
// 取消注册的方法
func (r *registry) remove(url string) error {
	// 遍历注册表中有没有服务地址，有则删除
	for k := range r.registrations {
		if r.registrations[k].ServiceURL == url {
			// 并发安全、删除对应url的访问地址
			r.mutex.Lock()
			r.registrations = append(r.registrations[:k], r.registrations[k+1:]...)
			r.mutex.Unlock()
			return nil
		}
	}
	return fmt.Errorf("service at URL %s not found",url)
}
```

决定在同一个路由下处理请求，在 ServerHttp 方法中添加取消注册请求的情况

```go
// 实现 ServerHttp 方法
func (s RegistryService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Request received")
	// 根据请求类型，进行不同处理逻辑
	switch r.Method {
	case http.MethodPost:
		dec := json.NewDecoder(r.Body)
		var r Registration
		err := dec.Decode(&r)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		log.Printf("Adding service: %v with URL: %s\n", r.ServiceName, r.ServiceURL)
		err = reg.add(r)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
        // 添加取消注册请求的情况
	case http.MethodDelete: 
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			log.Fatal(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		url := string(payload)
		log.Printf("Deleting service at URL: %s\n", url)
		err = reg.remove(url)
		if err != nil {
			log.Fatal(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}
```

在registry/client.go中添加方法：用于发送 取消服务 的请求

```go
// 向 registryservice 发送请求 取消服务
func ShutdownService(url string) error {
	// http包中没有delete方法: 对 服务注册 的 url 构建 delete 请求
	req, err := http.NewRequest(http.MethodDelete,ServicesURL,bytes.NewBuffer([]byte(url)))
	if err != nil {
		return nil
	}
    // 设置请求头
	req.Header.Add("Content-Type","text/plain")
    // 发送请求，获得响应
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	// 如果取消失败
	if res.StatusCode != http.StatusOK {
		// 错误小写开头--惯例
		return fmt.Errorf("failed to delete service. Delete service responded with code %v", res.StatusCode)
	}
	return nil

}
```

在service/service.go中的startService()方法中添加ShutdownService方法用于取消服务

```go
// 用于实际启动 HTTP 服务器
func startService(ctx context.Context, serviceName registry.ServiceName, host, port string) context.Context {

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
```

### 总结服务注册

以上，我学到了：

1. **编写独立的服务**：
   - 定义服务的基本信息（如服务名称和服务地址）。
   - 实现服务的处理逻辑（如HTTP处理程序）。
   - 定义服务的注册信息（如服务名称和服务URL）。
2. **启动独立的服务**：
   - ==创建新的日志记录器（如果需要）。==
   - 启动HTTP服务器来处理请求。
   - 调用`service.Start`函数来启动服务。
3. **将服务注册到服务注册中心**：（每个服务在启动时会向服务注册中心注册自己的信息，并在关闭时取消注册。）
   - 在服务启动时，向服务注册中心注册服务信息。
   - 在服务关闭时，向服务注册中心取消注册服务信息。

**从以下几个问题剖析：**

怎么编写一个服务？

怎么运行一个服务（怎么编写一个服务的运行程序）？

在分布式系统中，整体的架构是什么？服务是如何调用的？

怎么注册一个服务？

怎么取消一个服务？

**从开发不同阶段总结：**

==服务注册==

写一个服务注册服务（可运行的、可接收服务参数的后端接口程序）

```go
package main

import (
	"fmt"
	"net/http"
	"sync"
	"context"
)

// 服务注册的服务

// 1. 服务基本信息
const ServerPort = ":3000"
const ServerUrl = "http://localhost" + ServerPort + "/register"	// 其他服务 进行注册时的访问端口

type ServiceName string 

// 2. 定义服务处理handler
// 服务注册 服务
type RegisterService struct {}

// 根据下面的功能，我们准备定义一个注册记录类型和注册表类型
type Register struct{
	ServiceName ServiceName
	ServiceUrl string	
}

// 注册表
//type RegisterTable []Register
type RegisterTable struct {
	registerTable []Register
	registerTableLock *sync.Mutex		// 互斥锁
}

// 全局维护一个注册表
var registerTable = RegisterTable{
	registerTable : make([]Register,0),
	registerTableLock: new(sync.Mutex),
}

// 3. 定义服务功能handlerfunc：注册服务
func (reg RegisterService) ServeHTTP (w http.ResponseWriter,r *http.Request){
	// 根据请求找具体的处理函数
	fmt.Println("register service")
	switch r.Method {
		case "POST":
			// 注册服务
			registerTable.addService(w,r)
			fmt.Println("register service success")
		case "DELETE":
			fmt.Println("unregister service success")
		default:	
			w.WriteHeader(http.StatusNotFound)
			fmt.Println("unknown request method")
	}
}

// 注册服务功能，其实就是对注册表进行增加
// 有锁结构，不能是值传递，否则锁复制，锁失效
func (registerTable *RegisterTable)addService(_ http.ResponseWriter,r *http.Request) error {
	// 注册服务就是从请求中获取服务信息，然后保存到注册表中
	serviceName := r.FormValue("ServiceName")
	sserviceUrl := r.FormValue("ServiceUrl")
	registerTable.registerTableLock.Lock()
	defer registerTable.registerTableLock.Unlock()
	registerTable.registerTable = append(registerTable.registerTable, Register{
		ServiceName: ServiceName(serviceName),
		ServiceUrl:  sserviceUrl,
	})
	return nil
}

// 取消服务功能。。。

// 4. 启动服务（服务注册服务的启动是另外的）：为了确保其他服务可以注册成功，那么先启动服务注册功能
func main() {
	http.Handle("/register",RegisterService{})
	server := http.Server{Addr: ServerPort}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := server.ListenAndServe()
		if err != nil {
			fmt.Println("start server error:",err)
		}
		cancel()
	}()
	
	// 手动退出
	go func (){
		fmt.Println("Press Enter key to exit")
		buf := make([]byte, 1)
		fmt.Scanln(&buf)
		if buf != nil {
			cancel()
		}
	}()

	// 阻塞,收到退出信号时退出
	<-ctx.Done()
	fmt.Println("server exit")
}
```

写一个日志服务并注册到服务注册中心

```go
//由于需要注册其他服务，那么：
// 1. 还需要为其他服务调用后端服务准备“客户端”接口，让其他服务可以调用并注册自己：这里我们使用 发送http请求 来实现
// 2. 编写服务以及服务的运行程序
// 3. 其他服务的公共启动程序（2中提取出来，用于解耦）

// 为其他服务提供 发送http请求的方法接口

func Registerfunc(r Register) error{
	// json后的结构体
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	err := enc.Encode(&r)
	if err != nil {
		return err
	}
	// 发送http post请求：注册服务
	resp, err := http.Post(ServerUrl, "application/json", buf)

	// 请求过程的错误处理
	if err != nil {
		return err
	}
	// 响应的错误处理
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register service failed: %d",resp.StatusCode)
	}
	return nil
}
```

```go
// 编写新的服务

package new

import (
	"fmt"
	"net/http"
)

// 一个新的服务功能

func NewHandler(){
	http.HandleFunc("/new",func(w http.ResponseWriter, r *http.Request) {
		switch r.Method{
		case http.MethodPost:
			fmt.Println("Happy New Year!!!")
		default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
})
}

// 新的服务的启动程序
package main

import (
	"context"
	"distributed2/commonservice"
	"distributed2/new"
	"distributed2/register"
	"fmt"
	"log"
)

func main() {
	host, port := "localhost", "4000"
	reg := register.Register{
		ServiceName: register.ServiceName("New Service"),
		ServiceUrl:  "http://" + host + ":" + port,
	}
	// 启动服务
	ctx, err := commonservice.Start(context.Background(), host, port, reg, new.NewHandler)
	if err != nil {
        log.Fatalln(err)
    }

	<-ctx.Done()
    fmt.Println("Shutting down new service")
}
```

```go
// 其他服务公共的启动程序
package commonservice

import (
	"context"
	"distributed2/register"
	"fmt"
	"log"
	"net/http"
)

// 公共启动包

// 对于每一个服务：注册服务、启动服务
func Start(ctx context.Context,host,port string,reg register.Register,newHandler func()) (context.Context,error) {
	// 启动每个服务的路由
	newHandler()
	// 启动服务的http服务器
	ctx = startService(ctx, reg.ServiceName,host,port)
	// 注册服务
	err := register.Registerfunc(register.Register{ServiceName: reg.ServiceName, ServiceUrl: host + ":" + port})
	return ctx,err
}

// 用于实际启动 HTTP 服务器
func startService(ctx context.Context, serviceName register.ServiceName, host, port string) context.Context {

	// 可以创建一个可取消的上下文，通过传递取消信号来控制取消操作
	ctx,cancle := context.WithCancel(ctx)
	
	var server http.Server
	server.Addr = host + ":" + port
	
	// 启动 HTTP 服务器
	go func() {
		log.Println(server.ListenAndServe())

		// 关闭服务
		err := register.ShutdownService(fmt.Sprintf("http://%s:%s", host, port))
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
		// 关闭服务
		err := register.ShutdownService(fmt.Sprintf("http://%s:%s", host, port))
		if err != nil {
			log.Println(err)
		}
		server.Shutdown(ctx)
		cancle()
	}()

	return ctx
}
```

完善服务注册服务：完善服务取消注册功能

`略。。。`

其他独立服务...

`略。。。`

## 服务发现

1. `registration.go`新增服务注册的字段

```go
type Registration struct {
	ServiceName      ServiceName
	ServiceURL       string
	RequiredServices []ServiceName	// 用于存储服务的所有依赖服务
	ServiceUpdateURL string		// 更新url，用于发送服务注册的所有变动
}

// 一个记录，用于表示一个服务
type patchEntry struct {
	Name ServiceName
	URL  string
}

// 增加/删除记录（用于表示变化的服务集合）
type patch struct {
	Added   []patchEntry
	Removed []patchEntry
}
```

2. `server.go`在服务注册的时候添加服务的依赖项

```go
// 注册方法
func (r *registry) add(reg Registration) error {
	// 加锁解锁 ，避免并发问题

	r.mutex.Lock()
	
	// 增加服务记录到服务表
	r.registrations = append(r.registrations, reg)

	// 持久化注册信息
    err := r.saveToFile()
    if err != nil {
        return err
    }
	// 1 对应 1
	r.mutex.Unlock()

	// 在服务注册的时候添加服务的依赖项
	err = r.sendRequiredServices(reg)

	//r.mutex.Unlock()	// 一旦这里才解锁，服务注册只能完成第一个，其他的服务不能再注册了 并且 server.go 退出后，其他的也会退出
	// 总结：其实这里锁的位置只要保证：我在发送依赖信息的时候，确保没有写入 

	r.notify(patch{
		Added: []patchEntry{
			{
				Name: reg.ServiceName,
				URL:  reg.ServiceURL,
			},
		},
	})
	
	return err
}

// 发送依赖的服务
func (r registry) sendRequiredServices(reg Registration) error {
	// 1 对应 1 发送一个服务的依赖服务
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var p patch
	for _, serviceReg := range r.registrations {
		// 遍历服务注册表
		for _, reqService := range reg.RequiredServices {
			// 遍历该 服务 依赖的服务
			if serviceReg.ServiceName == reqService {
				// 注册表中有需要的依赖服务，将服务挂载到添加条目中
				p.Added = append(p.Added, patchEntry{serviceReg.ServiceName, serviceReg.ServiceURL})
			}
		}
	}
	err := r.sendPatch(p, reg.ServiceUpdateURL)
	if err != nil {
		return err
	}
	return nil
}

// 发送 服务依赖关系 的变动
func (r registry) sendPatch(p patch, url string) error {
	d, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = http.Post(url, "application/json", bytes.NewBuffer(d))
	if err != nil {
		return err
	}
	return nil
}
```

3. `registry/client.go`定义提供依赖服务的集合以及方法:

```go
//  提供依赖服务的集合
type providers struct {
	services map[ServiceName][]string
	mutex    *sync.RWMutex
}

var prov = providers{
	services: make(map[ServiceName][]string),
	mutex:    new(sync.RWMutex),
}

// 集合更新操作（根据传来的更新条目）：修改 服务的 `RequiredServices` 属性
func (p *providers) Update (pat patch) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, entry := range pat.Added {
		if _, ok := p.services[entry.Name]; !ok {
			p.services[entry.Name] = make([]string, 0)
		}
		p.services[entry.Name] = append(p.services[entry.Name], entry.URL)
	}

	for _, entry := range pat.Removed {
		if providerURLs, ok := p.services[entry.Name]; ok {
			for i, url := range providerURLs {
				if url == entry.URL {
					p.services[entry.Name] = append(providerURLs[:i], providerURLs[i+1:]...)
				}
			}
		}
	}
}

// 按道理，根据 ServiceName 获取对应的 providerURLs 是多个，返回的是一个slice
// 这里由于 其实 log 返回的只是 一个url
// 当然后面 web应用 在进行调用时也只获得一个服务地址，所以，后面的请求时通过 先获得url+手动拼凑路径
// 这也是“服务发现”的核心用法：web应用通过调用函数得到服务地址然后再请求数据。
func (p providers) get(name ServiceName) (string,error) {
	providers, ok := p.services[name]
	if !ok {
		return "", fmt.Errorf("no providers registered for %v", name)
	}
	idx := int(rand.Float32() * float32(len(providers)))
	return providers[idx], nil
}

// “服务发现” 对外暴露的函数：根据 ServiceName 获得 providerURLs
func GetProvider(service ServiceName) (string, error) {
	return prov.get(service)
}

// 处理刚才服务更新的post请求:
type serviceUpdateHandler struct {}

// 服务端有变化的时候，客户端接收更新
func (suh serviceUpdateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	dec := json.NewDecoder(r.Body)
	var p patch
	err := dec.Decode(&p)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	fmt.Printf("Updated received: %v\n", p)
	prov.Update(p)
}
```

4. `cmd/gradingservice/main.go`应该怎么利用“服务发现”使用自定义的日志记录服务

```go
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
	// 这里 通过服务名 获得 日志服务的服务url 
	if logProvider, err := registry.GetProvider(registry.LogService); err == nil {
		fmt.Printf("Logging service found at %s\n", logProvider)
        // 设置客户端的日志记录器：相当于可变的只有 reg.ServiceName，因为每个依赖日志服务的服务查询到的日志服务的地址是相同的。
		log.SetClientLogger(logProvider, reg.ServiceName)
	}
	// 接收服务关闭信号
	<-ctx.Done()
	fmt.Println("Shutting down grading service")
}
```

5. `registry/server.go`依赖变化时进行通知：

```go
// 比如：当 gradeservice 依赖 logservice 时，此时他们都启动了。突然，logservice 掉线了，但是 gradeservice 不知道。如果 logservice 服务又正常上线的话，其实问题不大。但是如果这时需要log服务处理来自 gradeservice 的请求，那么就会发生错误和不必要的请求。
// 所以：依赖变化时进行通知其实 就是 让依赖者及时并适当地处理这些变化，从而避免错误和不必要的请求。

func (r registry) notify(fullPatch patch) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, reg := range r.registrations {
		go func(reg Registration) {
			for _, reqService := range reg.RequiredServices {
				// 遍历 该服务的所有需要的服务
				p := patch{Added: []patchEntry{}, Removed: []patchEntry{}}
				sendUpdate := false
				// 遍历 一个条目的全部增加/删除的依赖记录
				for _, added := range fullPatch.Added {
					if added.Name == reqService {
						p.Added = append(p.Added, added)
						sendUpdate = true
					}
				}
				for _, removed := range fullPatch.Removed {
					if removed.Name == reqService {
						p.Removed = append(p.Removed, removed)
						sendUpdate = true
					}
				}

				if sendUpdate {
					err := r.sendPatch(p, reg.ServiceUpdateURL)
					if err != nil {
						log.Println(err)
						return
					}
				}
			}
		}(reg)
	}
}
```

6. `registry/client.go`中添加一句话表示收到服务更新状态信息

```go
//在 ServeHTTP 方法中
fmt.Printf("Updated received: %v\n", p)
```

## Web应用

其实就是简单的准备web的api 以及 处理的方法 以及 web应用的启动，`唯一不同的就是，web应用的数据处理通过请求注册中心的服务获得服务url，再用url向依赖的服务请求数据，整合后再响应。`

## 服务状态监控

对于每个注册到注册中心的服务，注册中心都要能监控每个服务的状态。那么，服务就需要为注册中心准备检查的url。

```go
HeartbeatURL:     serviceAddress + "/heartbeat"
// 这里就是每个服务的 服务地址 + /heartbeat 作为心跳检查的url
```

心跳检查 是 注册中心对所管理的服务进行的一项动作：

```go
// 注册中心 的动作；参数：间隔时间（比如间隔 3s 查询一次）
func (r *registry) heartbeat(freq time.Duration) {
    // 定时的操作就是 for死循环 + 时间间隔
	for {
        // 采用异步发送get请求的方式：
        // 1. 注册中心有多个服务，要对多个服务同时进行心跳检查（并发处理）
        // 2. 心跳检查需要发送网络请求，耗时可能很长，同步检查会影响整体服务的检查时间
        // 2. 并发处理使得如果一个服务出现耗时长或者阻塞的情况，不会影响其他服务的检查
		var wg sync.WaitGroup
		for _, reg := range r.registrations {
			wg.Add(1)
			go func(reg Registration) {
				defer wg.Done()
				success := true
                // 这里是一个重试机制：次数3次，间隔1s；只有检查正常则退出重试
				for attemps := 0; attemps < 3; attemps++ {
					res, err := http.Get(reg.HeartbeatURL)
					if err != nil {
						log.Println(err)
					} else if res.StatusCode == http.StatusOK {
						log.Printf("Heartbeat check passed for %v", reg.ServiceName)
						if !success {
                            // 重试成功，加入注册表
							r.add(reg)
						}
						break
					}
					log.Printf("Heartbeat check failed for %v", reg.ServiceName)
					if success {
                        // 检查失败，服务不健康，注册中心移除服务
						success = false
						r.remove(reg.ServiceURL)
					}
					time.Sleep(1 * time.Second)
				}
			}(reg) 
		}
		wg.Wait()
		time.Sleep(freq)
	}
}


var once sync.Once	// 提供一种机制，使得代码只执行一次

// 这里使用 go 开启一个协程是为了不阻塞主线程，而心跳函数中的协程是为了能并发处理心跳检查
// 提供给主线程，用于开启服务的心跳检查
func SetupRegistryService() {
	once.Do(func() {
		go reg.heartbeat(3 * time.Second)
	})
}
```

在registry/client.go的RegisterService方法中添加：

```go
// 心跳检查
	heartbeatURL, err := url.Parse(r.HeartbeatURL)
	if err != nil {
		return err
	}
	http.HandleFunc(heartbeatURL.Path, func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
```

cmd/registryservice/main.go中加入函数调用：

```go
// 函数调用，用于全局开启一次 心跳检查
registry.SetupRegistryService()
```

==后面的new是我用来练习的一个简单服务，主要用来理解服务编写、注册、调用的流程==
