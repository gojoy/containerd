package lazycopydir

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LazyReplicator struct {
	MonitorDir string // upperdir
	CrawlerDir string //nfsdir
	LazyDir    string //lazydir
	List       *JobList
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewLazyReplicator(crw, mon, lazy string) *LazyReplicator {

	list := NewJobList()

	ctx, cancel := context.WithCancel(context.Background())

	return &LazyReplicator{
		MonitorDir: mon,
		CrawlerDir: crw,
		LazyDir:    lazy,
		List:       list,
		ctx:        ctx,
		cancel:     cancel,
	}
}

func (l *LazyReplicator) CancelMonitor() {
	l.cancel()
}

func (l *LazyReplicator) Replicate() error {
	var (
		err             error
		sourcedir, file string
		targetdir       string
	)

	file, err = l.List.Pop()
	for err == nil {

		sourcedir = filepath.Join(l.CrawlerDir, file)
		targetdir = filepath.Join(l.LazyDir, file)

		if isdir(file) {
			_,err:=os.Stat(targetdir)
			if os.IsNotExist(err) {
				os.MkdirAll(targetdir,0755)
			}
		} else {
			if err = LocalCopy(sourcedir, targetdir); err != nil {
				glog.Println(err)
			}
		}

		file, err = l.List.Pop()
	}
	return nil
}

func isdir(v string)bool  {
	if v[len(v)-1]=='/' {
		return true
	}
	return false
}

func LocalCopy(source, target string) error {
	var (
		err      error
		src, dst *os.File
	)

	if _, err = os.Stat(target); err == nil {
		glog.Printf("target %v exist,don't copy\n", target)
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("Copy Error:%v\n", err)
	}
	if err = os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		glog.Println(err)
		return err
	}
	if src, err = os.Open(source); err != nil {
		glog.Println(err)
		return err
	}
	if dst, err = os.Create(target); err != nil {
		glog.Println(err)
		return err
	}
	defer func() {
		src.Close()
		dst.Close()
	}()

	_, err = io.Copy(dst, src)
	return err
}
