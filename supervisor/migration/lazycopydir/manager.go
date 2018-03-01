package lazycopydir

import "time"

func StartLazyCopy(crwdir, monidir, lazydir string) error {

	var (
		err error
	)

	replicator := NewLazyReplicator(crwdir, monidir, lazydir)

	if err = replicator.Crawler(); err != nil {
		glog.Println(err)
		return err
	}

	go func() {
		err = replicator.Monitor()
		if err != nil {
			glog.Println(err)
			panic(err)
		}
	}()

	if err = replicator.Replicate(); err != nil {
		glog.Println(err)
		panic(err)
	}

	replicator.CancelMonitor()

	time.Sleep(1 * time.Second)

	glog.Println("finish lazycopy!")
	return nil
}