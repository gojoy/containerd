package lazycopydir

import (
	"testing"

	"fmt"
)

func TestCrawlerAllFiles(t *testing.T) {

	l:=NewJobList()
	dir:="/tmp/testoverlay/lower"
	if err:=CrawlerAllFiles(dir,l);err!=nil {
		t.Error(err)
		return
	}
	l.Remove(".")
	fmt.Printf("l len is %d\n",len(l.data))
	v,err:=l.Pop()
	for err==nil {
		fmt.Printf("%s ",v)
		v,err=l.Pop()
	}
}


