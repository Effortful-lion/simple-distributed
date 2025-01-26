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
	log = stlog.New(fileLog(dest),"[go] - ",stlog.LstdFlags)
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