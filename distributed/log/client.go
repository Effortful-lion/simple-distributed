package log
import (
	"bytes"
	"distributed/registry"
	"fmt"
	"io"
	stlog "log"
	"net/http"
)

// 设置 日志记录器：给 具体的客户端（服务） 提供 日志服务
func SetClientLogger(serviceURL string, clientService registry.ServiceName) {
	stlog.SetPrefix(fmt.Sprintf("[%v] - ", clientService))
	stlog.SetFlags(0)
	stlog.SetOutput(&clientLogger{url: serviceURL})	// 自动调用write方法
}
type clientLogger struct {
	url string
}

// 这行代码确保 clientLogger 类型实现了 io.Writer 接口
var _ io.Writer = (*clientLogger)(nil)

// 将日志信息通过 HTTP POST 请求发送到指定的日志服务URL
func (cl clientLogger) Write(data []byte) (int, error) {
	b := bytes.NewBuffer([]byte(data))
	res, err := http.Post(cl.url+"/log", "text/plain", b)
	if err != nil {
		return 0, err
	}
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed to send log message. Service responed with code %v", res.StatusCode)
	}
	return len(data), nil
}
