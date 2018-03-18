package lazycopydir

import (
	"os"
	"log"
	"golang.org/x/sys/unix"
)

func isWhiteOut(name string) bool {
	var (
		stat unix.Stat_t
	)
	info,err:=os.Stat(name)
	if err!=nil {
		log.Println("file not exist")
		return false
	}
	log.Printf("%32b\n",info.Mode())
	log.Printf("%32b\n",os.ModeDevice)
	log.Printf("%32b\n",os.ModeCharDevice)
	if err=unix.Stat(name,&stat);err!=nil {
		log.Println(err)
	}

	log.Println(stat)
	return false
}
