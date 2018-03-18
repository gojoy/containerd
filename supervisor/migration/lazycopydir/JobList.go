package lazycopydir

import (
	"errors"
	"log"
	"strings"
	"sync"
)

var (
	JobListRemoveEmpty    = errors.New("Remove Error: Null JobList ")
	JobListRemoveNotFound = errors.New("Remove Error: Not Found ")
	JobListPopError       = errors.New("Pop Error: Null JobList ")
)

type JobList struct {
	w    sync.Mutex
	data []string
}

func NewJobList() *JobList {
	var (
		m sync.Mutex
		d = make([]string, 0)
	)
	return &JobList{
		w:    m,
		data: d,
	}
}

func (l *JobList) Append(v string) {
	l.w.Lock()
	defer l.w.Unlock()
	l.data = append(l.data, v)
}

func (l *JobList) Pop() (string, error) {
	l.w.Lock()
	defer l.w.Unlock()

	if len(l.data) == 0 {
		return "", JobListPopError
	}
	if len(l.data) == 1 {
		r := l.data[0]
		l.data = []string{}
		return r, nil
	}

	r := l.data[0]
	l.data = l.data[1:]
	return r, nil
}

func (l *JobList) Remove(v string) error {
	l.w.Lock()
	defer l.w.Unlock()

	if len(l.data) == 0 {
		return JobListRemoveEmpty
	}

	//log.Printf("list remove file %v\n", v)

	if len(l.data) == 1 {
		if l.data[0] == v {
			l.data = []string{}
			return nil
		} else {
			return JobListRemoveNotFound
		}
	}

	//delete v from slice
	for i, j := range l.data {
		if j == v {
			if i == 0 {
				l.data = l.data[1:]
			} else if i == len(l.data)-1 {
				l.data = l.data[:i-1]
			} else {
				//copy(l.data[i:], l.data[i+1:])
				//l.data=l.data[:len(l.data)-1]
				//just remove the last one to i,and delete the last one
				l.data[i] = l.data[len(l.data)-1]
				l.data = l.data[:len(l.data)-1]
			}

			return nil
		}
	}
	return JobListRemoveNotFound
}

//删除队列中该目录及该目录下的所有为文件
func (l *JobList) RemoveAllDir(v string) error {
	l.w.Lock()
	defer l.w.Unlock()

	length := len(l.data)

	if v[len(v)-1] != '/' {
		log.Printf("remove dir error: %v is not a dir\n", v)
	}
	log.Printf("list remove dir %v \n", v)
	if length == 0 {
		return JobListRemoveEmpty
	}

	if length == 1 {
		if l.data[0] == v {
			l.data = nil
			return nil
		} else {
			return JobListRemoveNotFound
		}
	}

	for i:=0;i<len(l.data);i++ {

		if strings.Contains(l.data[i], v) {
			//log.Printf("now i is %v,j is %v,length is %v\n",i,l.data[i],length)
			if i == 0 {
				l.data = l.data[1:]
			} else if i == len(l.data)-1 {
				l.data = l.data[:i-1]
			} else {
				//copy(l.data[i:], l.data[i+1:])
				//log.Printf("i is %v,len is %v\n", i, length)
				l.data[i] = l.data[length-1]
				l.data = l.data[:length-1]
			}
			i--
		}
		length = len(l.data)
	}
	return nil
}
