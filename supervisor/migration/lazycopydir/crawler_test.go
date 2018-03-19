package lazycopydir

import (
	"testing"

	"fmt"
	"os"
	"path/filepath"
	"io/ioutil"
	"log"
)

func TestCrawlerAllFiles(t *testing.T) {

	l := NewJobList()
	dir := "/var/lib/docker/workfile/overlaytest/lower"
	if err := CrawlerAllFiles(dir, l); err != nil {
		t.Error(err)
		return
	}
	//l.Remove(".")
	fmt.Printf("l len is %d\n", len(l.data))
	v, err := l.Pop()
	for err == nil {
		fmt.Printf("%s\n", v)
		v, err = l.Pop()
	}
}

func TestStartLazyCopy(t *testing.T) {
	var (
		err  error
		crw  = "/var/lib/docker/workfile/testoverlay/lower"
		mon  = "/var/lib/docker/workfile/testoverlay/upper"
		lazy = "/var/lib/docker/workfile/testoverlay/lazy"
	)
	go func() {
		if _, err = os.Create(filepath.Join(mon, "mount.txt")); err != nil {
			fmt.Println(err)
		}
	}()
	if err = StartLazyCopy(crw, mon, lazy); err != nil {
		t.Error(err)
		return
	}

	fmt.Println("lazycopy finish!")
}

func TestJobList_RemoveAllDir(t *testing.T) {
	var (
		err error
	)
	l := NewJobList()
	dir := "/var/lib/docker/workfile/vols/data"
	if err := CrawlerAllFiles(dir, l); err != nil {
		t.Error(err)
		t.FailNow()
		return
	}
	for _, v := range l.data {
		fmt.Println(v)
	}
	if err = l.RemoveAllDir("mysql/"); err != nil {
		t.Error(err)
		return
	}
	for _, v := range l.data {
		fmt.Println(v)
	}
	return

}

func TestIsWhiteOut(t *testing.T) {
	file := "/var/lib/docker/workfile/overlaytest/upper/b"
	//file1:="/dev/zero"
	fmt.Println(isWhiteOut(file))
}

func TestIsOpaque(t *testing.T) {
	dir := "/var/lib/docker/workfile/overlaytest/upper/add2"
	fmt.Println("isopaque:", isOpaque(dir))
	return
}

func TestHandleCreateEvents(t *testing.T) {
	log.SetFlags(log.Lshortfile)
	crawdir := "/var/lib/docker/workfile/overlaytest/lower"
	monidir:="/var/lib/docker/workfile/overlaytest/upper"
	l:=NewJobList()
	if err:=CrawlerAllFiles(crawdir,l);err!=nil {
		fmt.Println(err)
	}

	for _,v:=range l.data {
		fmt.Printf("list is %v\n",v)
	}
	println("begin to delete----------------")
	infos,err:=ioutil.ReadDir(monidir)
	if err!=nil {
		println(err)
		t.FailNow()
	}
	for _,v:=range infos {
		fmt.Printf("in test,file is %v~~~~~~~~~~~~~~~~~~\n",v.Name())
		if err:=HandleCreateEvents(filepath.Join(monidir,v.Name()),v.Name(),monidir,crawdir,l);err!=nil {
			println(err)
			t.FailNow()
		}
	}

	for _,v:=range l.data {
		fmt.Printf("list is %v\n",v)
	}
}