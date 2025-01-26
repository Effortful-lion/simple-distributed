package grades

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
)

// ctx context.Context, host, port string, reg registry.Registration,registerHandlersFunc func()

// 提供api
func RegisterHandlers() {
	handler := new(studentsHandler)
	http.Handle("/students", handler)
	http.Handle("/students/", handler)
}

// 提供 处理http请求的 处理器以及处理方法
type studentsHandler struct{}

// /students,/students/{id},/students/{id}/grades
func (sh studentsHandler) ServeHTTP(w http.ResponseWriter,r *http.Request){
	pathSegments := strings.Split(r.URL.Path, "/")
	switch len(pathSegments) {
	case 2:
		sh.getAll(w, r)
	case 3:
		id, err := strconv.Atoi(pathSegments[2])
		if err != nil{
			w.WriteHeader(http.StatusNotFound)
			return 
		}
		sh.getOne(w, r, id)
	case 4:
		id, err := strconv.Atoi(pathSegments[2])
		if err != nil{
			w.WriteHeader(http.StatusNotFound)
			return 
		}
		sh.addGrade(w, r, id)
	default:
		w.WriteHeader(http.StatusNotFound)
		return
	}
}

func (sh studentsHandler) getAll(w http.ResponseWriter,_ *http.Request) {
	studentsMutex.Lock()
	defer studentsMutex.Unlock()

	data, err := sh.toJSON(students)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type","application/json")
	w.Write(data)
}

// 转换json
func  (sh studentsHandler) toJSON (obj interface{}) ([]byte, error) {
	var buf bytes.Buffer	// 创建缓冲区
	encoder := json.NewEncoder(&buf)	// 创建json编码器，指向写入缓冲区
	err := encoder.Encode(obj)	// 编码并写入缓冲区
	if err != nil {
		return nil, fmt.Errorf("failed to serialize sudents: %q",err)
	}
	return buf.Bytes(), nil
}

// 这种不使用的参数就匿名
func (sh studentsHandler) getOne(w http.ResponseWriter,_ *http.Request, id int) {
	studentsMutex.Lock()
	defer studentsMutex.Unlock()

	student, err := students.GetByID(id)
	if err != nil{
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Failede to serilize student: %q", err)
		return 
	}

	data, err := sh.toJSON(student)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Add("Content-Type","application/json")
	w.Write(data)
}

// 加入某次成绩
func (sh studentsHandler) addGrade(w http.ResponseWriter,r *http.Request, id int) {
	studentsMutex.Lock()
	defer studentsMutex.Unlock()

	student, err := students.GetByID(id)
	if err != nil{
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("Failede to serilize student: %q", err)
		return 
	}

	var grade Grade
	decoder := json.NewDecoder(r.Body)
	err = decoder.Decode(&grade)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	student.Grades = append(student.Grades, grade)
	data, err := sh.toJSON(grade)
	if err != nil{
		log.Println(err)
		return
	}
	w.Header().Add("Content-Type","application/json")
	w.Write(data)
}