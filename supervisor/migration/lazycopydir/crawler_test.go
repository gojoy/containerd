package lazycopydir

import (
	"testing"

	"log"
	"path/filepath"
)

/*
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
	t.FailNow()
	return
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
	monidir := "/var/lib/docker/workfile/overlaytest/upper"
	l := NewJobList()
	if err := CrawlerAllFiles(crawdir, l); err != nil {
		fmt.Println(err)
	}

	for _, v := range l.data {
		fmt.Printf("list is %v\n", v)
	}
	println("begin to delete----------------")
	infos, err := ioutil.ReadDir(monidir)
	if err != nil {
		println(err)
		t.FailNow()
	}
	for _, v := range infos {
		fmt.Printf("in test,file is %v~~~~~~~~~~~~~~~~~~\n", v.Name())
		if err := HandleCreateEvents(filepath.Join(monidir, v.Name()), v.Name(), monidir, crawdir, l); err != nil {
			println(err)
			t.FailNow()
		}
	}

	for _, v := range l.data {
		fmt.Printf("list is %v\n", v)
	}
}



func TestLazyReplicator_Merge(t *testing.T) {
	log.SetFlags(log.Lshortfile|log.Ltime)
	var (
		err  error
		low,upper,merge,lazy string
		path = "/var/lib/migration/mvolume/" +
			"d5c02022a630311f4451dc89aec4257192751b944a7846fe8f9f51868ca93b08/0"
	)
	log.Println("begin to merge!------------------------")
	low=filepath.Join(path,"nfs")
	upper=filepath.Join(path,"upper")
	merge=filepath.Join(path,"merge")
	lazy=filepath.Join(path,"lazy")
	p:=NewLazyReplicator(low,upper,lazy,merge)

	if err=p.Finish();err!=nil {
		log.Println(err)
		t.FailNow()
		return
	}
	return
}

*/

func TestMergeDir(t *testing.T) {
	log.SetFlags(log.Lshortfile | log.Ltime)
	var (
		err                error
		upper, merge, lazy string
		path               = "/var/lib/migration/mvolume/" +
			"d5c02022a630311f4451dc89aec4257192751b944a7846fe8f9f51868ca93b08/1"
	)
	log.Println("begin to merge!------------------------")
	//low=filepath.Join(path,"nfs")
	upper = filepath.Join(path, "upper")
	merge = filepath.Join(path, "merge")
	lazy = filepath.Join(path, "lazy")
	if err = MergeDir(upper, lazy, merge); err != nil {
		log.Println(err)
		t.FailNow()
	}
	return
}
