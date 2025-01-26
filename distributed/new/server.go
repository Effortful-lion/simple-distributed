package new

import (
	"bytes"
	"distributed/registry"
	"encoding/json"
	"fmt"
	"net/http"
)

func RegisterHandlers() {
	http.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			// 记录日志消息到日志服务
			WritetoLog(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	})
}

func WritetoLog(w http.ResponseWriter, r *http.Request) {
	// 从json中获得信息
	var requestData struct {
		Msg string `json:"msg"`
	}
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&requestData)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	msg := requestData.Msg
	fmt.Println("msg:", msg)

	// 通过服务发现 得到日志服务地址
	url, err := registry.GetProvider(registry.LogService)
	fmt.Println("url:", url)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if url == "" {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// 向日志服务发送http post请求信息
	body := bytes.NewBufferString(msg)
	_, err = http.Post(url+"/log", "text/plain", body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}