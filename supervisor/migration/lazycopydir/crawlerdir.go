package lazycopydir

import (
	"os"

	"log"
	"path/filepath"
)

var (
	glog *log.Logger
)

func init() {
	glog = log.New(os.Stderr, "lazyCopyLog: ", log.Lshortfile)
}

func (l *LazyReplicator) Crawler() error {

	if len(l.List.data) > 0 {
		glog.Println("crawler should start with empty joblist")
		_, err := l.List.Pop()
		for err != JobListPopError {
			_, err = l.List.Pop()
		}
	}

	return CrawlerAllFiles(l.CrawlerDir, l.List)
}

func CrawlerAllFiles(dir string, list *JobList) error {

	var (
		err    error
		tmpdir string
	)

	if tmpdir, err = os.Getwd(); err != nil {
		glog.Println(err)
		return err
	}

	defer func() {
		if err = os.Chdir(tmpdir); err != nil {
			glog.Fatalln(err)
		}
	}()

	if err = os.Chdir(dir); err != nil {
		glog.Println(err)
		return err
	}

	//add all crwfiles to list
	if err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {

		if err != nil {
			glog.Println(err)
			return err
		}
		//if !info.IsDir() {
		//	list.Append(path)
		//}
		list.Append(path)
		return nil
	}); err != nil {
		glog.Println(err)
		return err
	}
	return nil
}
