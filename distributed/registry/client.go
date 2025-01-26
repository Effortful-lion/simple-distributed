package registry

// 相当于一个客户端（为客户端提供函数 向注册服务（registryservice）发送请求以注册一个服务）
// 内部发送http请求，用于进行服务注册
import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
)

// 向 registryservice 发送请求 注册服务
func RegisterService(r Registration) error {

	// 这里作为接收者，处理http请求：为了确保服务在注册时能够立即验证其健康状态，并且确保服务在运行期间能够持续监控其健康状态。
	// 心跳检查
	heartbeatURL, err := url.Parse(r.HeartbeatURL)
	if err != nil {
		return err
	}
	http.HandleFunc(heartbeatURL.Path, func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})

	// 服务更新
	serviceUpdateURL, err := url.Parse(r.ServiceUpdateURL)
	if err != nil {
		return err
	}
	http.Handle(serviceUpdateURL.Path, &serviceUpdateHandler{})

	// 创建 JSON 编码器

	// 创建一个新的缓冲区 buf ，缓冲区实现了io.Writer 和 io.Reader可以进行数据的读取和写入
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	err = enc.Encode(r)
	if err != nil {
		return nil
	}
	
	res, err := http.Post(ServicesURL,"application/json", buf)
	// 请求过程中的错误：表示请求成功发送，但你仍然需要检查 res.StatusCode
	if err != nil {
		return err
	}

	// 处理过程中的错误
	// 如果注册失败
	if res.StatusCode != http.StatusOK {
		// 错误小写开头--惯例
		return fmt.Errorf("failed to register service. Registry service responded with code %v", res.StatusCode)
	}

	return nil
}

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

// 向 registryservice 发送请求 取消服务
func ShutdownService(url string) error {
	// http包中没有delete方法: 对 服务注册 的 url 构建 delete 请求
	req, err := http.NewRequest(http.MethodDelete,ServicesURL,bytes.NewBuffer([]byte(url)))
	if err != nil {
		return nil
	}
	req.Header.Add("Content-Type","text/plain")
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

//  提供依赖服务的集合
type providers struct {
	services map[ServiceName][]string
	mutex    *sync.RWMutex
}

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
func (p providers) get(name ServiceName) (string,error) {
	providers, ok := p.services[name]
	if !ok {
		return "", fmt.Errorf("no providers registered for %v", name)
	}
	idx := int(rand.Float32() * float32(len(providers)))
	return providers[idx], nil
}

// 对外暴露的函数：根据 ServiceName 获得 providerURLs
func GetProvider(service ServiceName) (string, error) {
	return prov.get(service)
}

var prov = providers{
	services: make(map[ServiceName][]string),
	mutex:    new(sync.RWMutex),
}