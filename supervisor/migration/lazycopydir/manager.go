package lazycopydir

import (
	"log"
	"time"
)

//crwdir 挂载的nfs目录 monidiruper读写层目录 lazydir 惰复制目录
func StartLazyCopy(crwdir, monidir, lazydir string) error {

	var (
		err error
	)

	replicator := NewLazyReplicator(crwdir, monidir, lazydir, "./")

	if err = replicator.Crawler(); err != nil {
		log.Println(err)
		return err
	}

	go func() {
		err = replicator.Monitor()
		if err != nil {
			log.Println(err)
			panic(err)
		}
	}()

	if err = replicator.Replicate(); err != nil {
		log.Println(err)
		panic(err)
	}

	replicator.CancelMonitor()

	time.Sleep(1 * time.Second)

	log.Println("finish lazycopy!")
	return nil
}

func (replicator *LazyReplicator) StartCrawler() error {
	var (
		err error
	)

	if err = replicator.Crawler(); err != nil {
		log.Println(err)
		return err
	}
	log.Printf("crawler %v finish,len is %v\n", replicator.CrawlerDir, len(replicator.List.data))
	for i, v := range replicator.List.data {
		log.Printf("%v:%v\n", i, v)
	}
	return nil
}

func (replicator *LazyReplicator) StartMonitor() error {
	var (
		err error
	)
	log.Println("now begin monitor! ")
	go func() {
		err = replicator.Monitor()
		if err != nil {
			log.Println(err)
			panic(err)
		}
	}()

	return nil

}

func (replicator *LazyReplicator) Dolazycopy() error {

	var (
		err error
	)

	if err = replicator.Replicate(); err != nil {
		log.Println(err)
		panic(err)
	}

	replicator.CancelMonitor()

	time.Sleep(1 * time.Second)

	log.Println("finish lazycopy!")

	return nil
}

func (r *LazyReplicator) Umount() error {
	var (
		err error
	)
	if err = UmountDir(r.CrawlerDir); err != nil {
		log.Println(err)
		return err
	}
	if err = UmountDir(r.Mgergedir); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (r *LazyReplicator) Merge() error {
	if err := MergeDir(r.MonitorDir, r.LazyDir, r.Mgergedir); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (r *LazyReplicator) Finish() error {
	var (
		err error
	)
	if err = r.Umount(); err != nil {
		log.Printf("finish lazyrepilcator failed:%v\n", err)
		return err
	}
	if err = r.Merge(); err != nil {
		log.Printf("finish lazyreplicator failed:%v\n", err)
		return err
	}
	return nil
}
