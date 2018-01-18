package lazycopydir

import (
	"testing"

	"fmt"
	"os"
	"path/filepath"
)

func TestCrawlerAllFiles(t *testing.T) {

	l:=NewJobList()
	dir:="/tmp/testoverlay/lower"
	if err:=CrawlerAllFiles(dir,l);err!=nil {
		t.Error(err)
		return
	}
	l.Remove(".")
	fmt.Printf("l len is %d\n",len(l.data))
	v,err:=l.Pop()
	for err==nil {
		fmt.Printf("%s ",v)
		v,err=l.Pop()
	}
}

func TestStartLazyCopy(t *testing.T) {
	var (
		err error
		crw="/var/lib/docker/workfile/testoverlay/lower"
		mon="/var/lib/docker/workfile/testoverlay/upper"
		lazy="/var/lib/docker/workfile/testoverlay/lazy"
	)
	go func() {
		if _,err=os.Create(filepath.Join(mon,"mount.txt"));err!=nil {
			fmt.Println(err)
		}
	}()
	if err=StartLazyCopy(crw,mon,lazy);err!=nil {
		t.Error(err)
		return
	}

	fmt.Println("lazycopy finish!")
}
