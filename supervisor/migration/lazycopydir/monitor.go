package lazycopydir

import (
	"context"
	"github.com/fsnotify/fsnotify"
	"log"
	"os"
	"path/filepath"
)

func (l *LazyReplicator) Monitor() error {
	return MonitorDir(l.MonitorDir, l.List, l.ctx, l.CrawlerDir)
}

//dir:monitor(upperdir)
func MonitorDir(dir string, list *JobList, ctx context.Context, crawdir string) error {
	var (
		err     error
		w       *fsnotify.Watcher
		addpath string
	)

	if w, err = fsnotify.NewWatcher(); err != nil {
		log.Println(err)
		return err
	}

	//monitor monidir,remove updated file from list
	if err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Println(err)
			return err
		}
		//add all dir to watch list
		if info.IsDir() {
			if err = w.Add(path); err != nil {
				log.Println(err)
				return err
			}
			log.Printf("add %v to monitor lists\n", path)
		}

		return nil
	}); err != nil {
		log.Println(err)
		return err
	}

	//开始监控目录，当有新的文件夹创建时，加入监控列表 并且及时的从队列中删除
	for len(list.data) > 0 {
		select {
		case events := <-w.Events:
			//处理新建文件
			if events.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("watch create %v\n", events.Name)
				info, err := os.Stat(events.Name)
				if err == nil && info.IsDir() {
					if err = w.Add(events.Name); err != nil {
						log.Println(err)
					}
					log.Printf("now update watch dir add %v\n", events.Name)
				}

				if addpath, err = filepath.Rel(dir, events.Name); err != nil {
					log.Printf("Rel err:%v\n", err)
				} else {

					if err=HandleCreateEvents(events.Name,addpath,dir,crawdir,list);err!=nil {
						log.Println(err)
					}

					//if isDirInPath(addpath, crawdir) {
					//
					//	deletedir := addpath + "/"
					//	list.RemoveAllDir(deletedir)
					//
					//} else {
					//
					//	if err = list.Remove(addpath); err != nil {
					//		log.Printf("remove %v,err:%v\n", addpath, err)
					//	}
					//}
					//log.Printf("remove %v,now len is %v\n", addpath, len(list.data))

				}
			}

			if events.Op&fsnotify.Remove == fsnotify.Remove ||
				events.Op&fsnotify.Rename == fsnotify.Rename {
				info, err := os.Stat(events.Name)
				if err == nil && info.IsDir() {
					if err = w.Remove(events.Name); err != nil {
						log.Println(err)
					}
				}

			}

		case err = <-w.Errors:
			if err != nil {
				//只打印出错误，继续监视
				log.Println(err)
			}
		case <-ctx.Done():
			log.Println("receive done,Exist Monitor")
			goto End
		}
	}

	goto End

End:
	log.Println("Monitor End")
	return w.Close()
}
