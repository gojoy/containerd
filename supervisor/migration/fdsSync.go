package migration


import (
	"os/exec"
	"bufio"
	"bytes"
	"strings"
	"os"
	"log"
	"path/filepath"
	"encoding/json"
)

const openFileDir="/run/migration/openfile"


//首先把数据卷拷贝到对应的远程upper目录，然后根据crit x fds文件列表，同步这些文件
//在本地监控数据卷，生成map 所有变更都存在其中，然后把上次同步过的文件从map中删除，
// 之后在目的主机的upper目录，文件只有在map中，表示以及不是最新版本，就删除


// 0: local src copy dir  1: local relative file path
//[/var/lib//docker../data t1/t1.ibd]
type allpath [2]string
// each volumes dir,need to sync files
type volpath []allpath

// remoteVolumesDirs:代表每个数据卷在目的主机的拷贝目录 /var/lib/migration/mvolums/0/upper
func fdsSync(checkpointDir string,remoteVolumesDirs []string,vols []Volumes,ip string) error {
	files,err:=getFiles(checkpointDir)
	if err!=nil {
		log.Println(err)
		return err
	}
	syncfiles,err:=syncNeedFiles(files,vols)
	if err!=nil {
		log.Println(err)
		return err
	}
	if err=fastCopy(syncfiles,ip,remoteVolumesDirs);err!=nil {
		log.Println(err)
		return err
	}
	return nil
}

//将打开的文件保存到/run/migration/openfile目录下
func SaveOpenFile(checkdir, id string,vol []Volumes) error {

	files,err:=getFiles(checkdir)
	if err!=nil {
		log.Println(err)
		return err
	}
	data,err:=syncNeedFiles(files,vol)
	if err!=nil {
		log.Println(err)
		return err
	}

	err=dumpFiles(data,id)
	if err!=nil {
		log.Println(err)
		return err
	}
	return nil
}

func dumpFiles(data []volpath,id string) error {
	if len(data)==0 {
		log.Println("open file is nil")
		return nil
	}
	openjsondir:=filepath.Join(openFileDir,id)
	openjson:=filepath.Join(openjsondir,"open.json")
	err:=os.MkdirAll(openjsondir,0665)
	if err!=nil {
		log.Println(err)
		return err
	}
	f,err:=os.Create(openjson)
	if err!=nil {
		log.Println(err)
		return err
	}
	defer f.Close()
	enc:=json.NewEncoder(f)
	enc.SetIndent("","	")
	for _,v:=range data {
		if err=enc.Encode(v);err!=nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

//获得所有打开的文件
func getFiles(path string) ([]string,error) {
	var (
		err error
		res=make([]string,0)
	)

	args:=[]string{"x"}
	args=append(args,path,"fds")
	cmd:=exec.Command("crit",args...)
	out,err:=cmd.Output()
	log.Println(cmd.Args)
	if err!=nil {
		log.Println(err,string(out))
		return res,err
	}
	//log.Printf("out is %v\n",string(out))

	s:=bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		txt:=s.Text()
		//log.Println(s.Text())
		sp:=strings.Split(txt,":")
		//log.Printf("sp is %v,len is %v\n",sp,len(sp))
		if len(sp)==2 {
			if sp[1][1]=='/' {
				res=append(res,sp[1][1:])
			}
		}
	}
	res=res[:len(res)-2]
	//log.Printf("res is %v,len is %v\n",res,len(res))
	return res,nil
}

//找到再数据卷中的文件
func syncNeedFiles(files []string, vol []Volumes) ([]volpath,error) {
	var (
		err error
		realp=make([]allpath,0)
		res=make([]volpath,0)
	)
	if len(vol) == 0 {
		log.Println("vol NULL")
		return res,nil
	}

	for _, v := range vol {
		if !v.isWrite {
			continue
		}
		for i := 0; i < len(files); i++ {
			right:=strings.TrimPrefix(files[i],v.dst)
			if len(right)!=len(files[i]) {
				//log.Printf("path is %v,right is %v\n",files[i],right)
				//l:=filepath.Join(v.src,right)
				//log.Printf("copy file is %v\n",l)
				realp=append(realp,allpath{v.src,right})
			}
		}
		res=append(res,realp)
	}
	//log.Printf("len is %v\n",len(res))
	return res,err
}

func fastCopy(files []volpath, ip string, remote []string) error {
	var (
		err error
	)
	if len(files)==0 {
		return err
	}

	//log.Printf("f is %v\n",files)
	for i,v:=range files {
		if err=os.Chdir(v[0][0]);err!=nil {
			log.Println(err)
		}
		for _,v1:=range v {
			if err=RemoteCopyFileRsync(v1[1],remote[i],ip);err!=nil {
				log.Println(err)
			}
		}

	}
	return nil
}

func RemoteCopyFileRsync(local, remote string, ip string) error {

	var (
		err error
	)
	//if local[len(local)-1] != '/' {
	//	local = local + "/"
	//}
	//if remote[len(remote)-1] != '/' {
	//	remote = remote + "/"
	//}

	args := append([]string{"-azR"}, local, "root@"+ip+":"+remote)
	//log.Printf("l is %v,r is %v,args is %v\n",local,remote,args)

	cmd := exec.Command("rsync", args...)
	log.Println(cmd.Args)
	//log.Printf("cmd is %v\n",cmd)
	if out,err:= cmd.CombinedOutput(); err != nil {
		log.Printf("rsync error:%v,out:%v\n", err,string(out))
		log.Printf("cmd is %v\n", cmd.Args)
	}
	return err
}
