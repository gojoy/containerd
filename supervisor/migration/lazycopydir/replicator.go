package lazycopydir


type LazyReplicator struct {
	MonitorDir string
	CrawlerDir string
	LazyDir string
	List *JobList

}

func NewLazyReplicator(mon,crw,lazy string) *LazyReplicator  {

	list:=NewJobList()

	return &LazyReplicator{
		MonitorDir:mon,
		CrawlerDir:crw,LazyDir:lazy,
		List:list,
	}
}

func (l *LazyReplicator) Replicate() error  {
	var (
		err error
		source string
	)
	source,err=l.List.Pop()
	for err==nil {
		println(source)
	}
	return nil

}
