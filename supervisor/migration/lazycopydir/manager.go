package lazycopydir

import (
	"time"
	"log"
)

//crwdir 挂载的nfs目录 monidiruper读写层目录 lazydir 惰复制目录
func StartLazyCopy(crwdir, monidir, lazydir string) error {

	var (
		err error
	)

	replicator := NewLazyReplicator(crwdir, monidir, lazydir)

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

func (replicator *LazyReplicator) Prelazy() error {
	var (
		err error
	)

	if err = replicator.Crawler(); err != nil {
		log.Println(err)
		return err
	}
	log.Printf("crawler %v finish,len is %v\n",replicator.CrawlerDir,len(replicator.List.data))
	for _,v:=range replicator.List.data {
		log.Println(v)
	}
	log.Println(" ")
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
