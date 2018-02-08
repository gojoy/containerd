package lazycopydir

import (
	"context"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"os"
	"path/filepath"
)

func (l *LazyReplicator) Monitor() error {
	return MonitorDir(l.MonitorDir, l.List, l.ctx)
}

func MonitorDir(dir string, list *JobList, ctx context.Context) error {
	var (
		err     error
		w       *fsnotify.Watcher
		listdir string
	)

	if w, err = fsnotify.NewWatcher(); err != nil {
		glog.Println(err)
		return err
	}

	if err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			glog.Println(err)
			return err
		}
		if err = w.Add(path); err != nil {
			glog.Println(err)
			return err
		}
		return nil
	}); err != nil {
		glog.Println(err)
		return err
	}

	for len(list.data) > 0 {
		select {
		case events := <-w.Events:
			if events.Op&fsnotify.Create == fsnotify.Create {
				if err = w.Add(events.Name); err != nil {
					glog.Println(err)
				}
				if listdir, err = filepath.Rel(dir, events.Name); err != nil {
					glog.Printf("Rel err:%v\n", err)

				} else {
					if err = list.Remove(listdir); err != nil {
						glog.Println(err)
					}
				}
			}
			if events.Op&fsnotify.Remove == fsnotify.Remove ||
				events.Op&fsnotify.Rename == fsnotify.Rename {
				if err = w.Remove(events.Name); err != nil {
					glog.Println(err)
				}
			}

		case err = <-w.Errors:
			if err != nil {
				//只打印出错误，继续监视
				glog.Println(err)
			}
		case <-ctx.Done():
			glog.Println("Exist Monitor")
			goto End
		}
	}

	goto End

End:
	glog.Println("Monitor End")
	return w.Close()
}
