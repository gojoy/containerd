package lazycopydir

import (
	"testing"
	"os"
	"fmt"
	"path/filepath"
)

func TestCrawlerAllFiles(t *testing.T) {
	d,err:=os.Getwd()
	if err!=nil {
		t.Error(err)
		return
	}
	fmt.Printf("d is %v,\ndir is %v\n",d,filepath.Dir(d))
}
