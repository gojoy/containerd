package migration

import (
	"path/filepath"
	"os/exec"
	"log"
	"strconv"
)

const minMem=20*1024
const maxTimes=10
const netSpeed=1024*30
const PreDump="predump"


func preCopy(id int) error  {
	var (
		err error
		base,path,parent string
		i int
	)
	base=filepath.Join(MigrationDir,PreDump,strconv.Itoa(id))
	path=filepath.Join(base,"0")
	if err=doFirstPreDump(id,path);err!=nil {
		log.Println(err)
		return err
	}
	for i=1;i<maxTimes;i++ {
		parent=path
		path=filepath.Join(base,strconv.Itoa(i))
		if err=doNPreDump(id,path,parent);err!=nil {
			log.Println(err)
			return err
		}

	}
	parent=path
	path=filepath.Join(base,"last")
	if err=doLastDump(id,path,parent);err!=nil {
		log.Println(err)
		return err
	}
	return nil
}

func doFirstPreDump(id int, path string) error {
	var (
		err error
	)

	args:=[]string{"checkpoint","--pre-dump","tcp-established"}
	args=append(args,"--image-path",path,"--work-path",path,strconv.Itoa(id))
	cmd:=exec.Command("runc",args...)
	out,err:=cmd.CombinedOutput()
	if err!=nil {
		log.Printf("err:%v,out:%v\n",err,string(out))
		return err
	}
	return nil
}

func doNPreDump(id int,path,parent string) error  {
	var (
		err error
	)
	args:=[]string{"checkpoint","--pre-dump","tcp-established"}
	args=append(args,"--image-path",path,"--work-path",path,"--parent-path",parent,strconv.Itoa(id))
	cmd:=exec.Command("runc",args...)
	out,err:=cmd.CombinedOutput()
	if err!=nil {
		log.Printf("err:%v,out:%v\n",err,string(out))
		return err
	}
	return nil
}

func doLastDump(id int,path,parent string)error  {
	var (
		err error
	)
	args:=[]string{"checkpoint","tcp-established"}
	args=append(args,"--image-path",path,"--work-path",path,"--parent-path",parent,strconv.Itoa(id))
	cmd:=exec.Command("runc",args...)
	out,err:=cmd.Output()
	if err!=nil {
		log.Printf("err:%v,out:%v\n",err,string(out))
		return err
	}
	return nil
}