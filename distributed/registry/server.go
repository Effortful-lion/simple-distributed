package registry

// 服务注册的 service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// 服务注册的ip和端口
const ServerPort = ":3000"
const ServicesURL = "http://localhost" + ServerPort + "/services" // 通过这个url可以查询到哪些服务？
const registryFile = "./registry.json"

// 服务注册中心（相当于服务注册的表：记录1--服务1，记录2--服务2...）
type registry struct {
	registrations []Registration // 已经注册的服务（可能多个线程并发访问，并且是动态变化的，所以要确保并发安全性）
	mutex         *sync.RWMutex    // 互斥锁
}

// 持久化注册信息到文件
func (r *registry) saveToFile() error {
    file, err := os.Create(registryFile)
    if err != nil {
        return err
    }
    defer file.Close()

    encoder := json.NewEncoder(file)
    return encoder.Encode(r.registrations)
}

// 从文件中恢复注册信息
func (r *registry) loadFromFile() error {
    file, err := os.Open(registryFile)
    if err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return err
    }
    defer file.Close()

    decoder := json.NewDecoder(file)
    return decoder.Decode(&r.registrations)
}

// 向外提供的加载文件的接口
func LoadFromFile() error {
	return reg.loadFromFile()
}

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

	// TODO(理解为什么要这一步，有什么作用？):注册服务后，发送所有依赖的服务
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

func (r *registry) heartbeat(freq time.Duration) {
	for {
		var wg sync.WaitGroup
		//wg.Add(len(r.registrations))	// 也可以放在外面，但是这里放在里面
		for _, reg := range r.registrations {
			wg.Add(1)
			go func(reg Registration) {
				defer wg.Done()
				success := true
				for attemps := 0; attemps < 3; attemps++ {
					res, err := http.Get(reg.HeartbeatURL)
					if err != nil {
						log.Println(err)
					} else if res.StatusCode == http.StatusOK {
						log.Printf("Heartbeat check passed for %v", reg.ServiceName)
						if !success {
							r.add(reg)
						}
						break
					}
					log.Printf("Heartbeat check failed for %v", reg.ServiceName)
					if success {
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
var once sync.Once

// 这里使用 go 开启一个协程是为了不阻塞主线程，而心跳函数中的协程是为了能并发处理心跳检查
func SetupRegistryService() {
	once.Do(func() {
		go reg.heartbeat(3 * time.Second)
	})
}

// 发送服务状态更新通知
func (r registry) notify(fullPatch patch) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, reg := range r.registrations {
		go func(reg Registration) {
			for _, reqService := range reg.RequiredServices {
				// 遍历 该服务的所有需要的服务
				// 为每个依赖服务初始化一个补丁p，并设置一个标志sendUpdate来决定是否需要发送更新。
				p := patch{Added: []patchEntry{}, Removed: []patchEntry{}}
				sendUpdate := false
				// 遍历fullPatch中的所有新增服务条目，如果当前服务需要该新增服务，则将其添加到补丁p中，并设置更新标志。
				for _, added := range fullPatch.Added {
					if added.Name == reqService {
						p.Added = append(p.Added, added)
						sendUpdate = true
					}
				}
				// 遍历fullPatch中的所有删除服务条目，如果当前服务需要该删除服务，则将其添加到补丁p中，并设置更新标志。
				for _, removed := range fullPatch.Removed {
					if removed.Name == reqService {
						p.Removed = append(p.Removed, removed)
						sendUpdate = true
					}
				}
				// 如果需要发送更新，则调用sendPatch方法发送补丁p到当前服务的更新URL。
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

// 取消注册的方法
func (r *registry) remove(url string) error {
	// 遍历注册表中有没有服务地址，有则删除
	for k := range r.registrations {
		if r.registrations[k].ServiceURL == url {
			r.notify(patch{Removed: []patchEntry{{r.registrations[k].ServiceName, r.registrations[k].ServiceURL}}},)
			// 并发安全、删除对应url的访问地址
			r.mutex.Lock()
			r.registrations = append(r.registrations[:k], r.registrations[k+1:]...)
			r.mutex.Unlock()
			return nil
		}
	}
	return fmt.Errorf("service at URL %s not found",url)
}

// 定义服务表结构（服务数0，互斥锁）
var reg = registry{
	registrations: make([]Registration, 0),
	mutex:         new(sync.RWMutex),
}

// 定义 注册服务的实现逻辑（handler）
type RegistryService struct{}

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
