package lazycopydir

import (
	"os"

	"log"
	"path/filepath"
)

//var (
//	log *log.Logger
//)
//
//func init() {
//	log = log.New(os.Stderr, "lazyCopyLog: ", log.Lshortfile)
//}

func (l *LazyReplicator) Crawler() error {

	if len(l.List.data) > 0 {
		log.Println("crawler should start with empty joblist")
		l.List.data = []string{}
	}

	return CrawlerAllFiles(l.CrawlerDir, l.List)
}

func CrawlerAllFiles(dir string, list *JobList) error {

	var (
		err    error
		tmpdir string
	)

	if tmpdir, err = os.Getwd(); err != nil {
		log.Println(err)
		return err
	}

	defer func() {
		if err = os.Chdir(tmpdir); err != nil {
			log.Fatalln(err)
		}
	}()

	if err = os.Chdir(dir); err != nil {
		log.Println(err)
		return err
	}

	//add all crwfiles to list
	if err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {

		if err != nil {
			log.Println(err)
			return err
		}
		//log.Printf("path is %v,info is %v\n",path,info.Name())
		if info.IsDir() {
			list.Append(path + "/")
		} else {
			list.Append(path)
		}
		return nil
	}); err != nil {
		log.Println(err)
		return err
	}
	list.Pop()
	return nil
}
