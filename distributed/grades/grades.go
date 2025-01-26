package grades

// 业务服务：关于学生的成绩管理（相当于一个model、dao）

import (
	"fmt"
	"sync"
)

type Student struct {
	ID        int
	FirstName string
	LastName  string
	Grades    []Grade
}

type GradeType string

type Grade struct {
	Title string
	Type  GradeType
	Score float32
}

const (
	GradeQuiz = GradeType("Quiz") // 小型考试
	GradeTest = GradeType("Test") // 测试
	GradeExam = GradeType("Exam") // 大型考试
)

// 计算一个学生的平均成绩
func (s Student) Average() float32 {
	var result float32
	for _, grade := range s.Grades {
		result += grade.Score
	}

	return result / float32(len(s.Grades))
}

type Students []Student

// 学生集合变量
var (
	students Students
	studentsMutex sync.Mutex
)

// 根据学生id查询学生
func (ss Students) GetByID(id int) (*Student, error) {
	for i := range ss {
		if ss[i].ID == id {
			return &ss[i], nil
		}
	}
	return nil, fmt.Errorf("student with ID %d not found", id)
}