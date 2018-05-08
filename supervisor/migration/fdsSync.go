package migration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

//首先把数据卷拷贝到对应的远程upper目录，然后根据crit x fds文件列表，同步这些文件
//在本地监控数据卷，生成map 所有变更都存在其中，然后把上次同步过的文件从map中删除，
// 之后在目的主机的upper目录，文件只有在map中，表示以及不是最新版本，就删除

// 0: local src copy dir  1: local relative file path
//[/var/lib//docker../data t1/t1.ibd]
type allpath [2]string

// each volumes dir,need to sync files
type volpath []allpath

// remoteVolumesDirs:代表每个write数据卷在目的主机的拷贝目录
func fdsSync(checkpointDir string, remoteVolumesDirs string, vols Volumes, ip string, smap map[string]bool) error {
	files, err := getFiles(checkpointDir)
	if err != nil {
		log.Println(err)
		return err
	}
	syncfiles := syncNeedFiles(files, vols)
	if err != nil {
		log.Println(err)
		return err
	}

	log.Printf("before delete,len is %v,syncfiles is %v", len(syncfiles), syncfiles)
	syncfiles = deletefromstablemap(syncfiles, smap)
	log.Printf("now len is %v,syncfiles  is %v\n", len(syncfiles), syncfiles)

	if err = fastCopy(syncfiles, ip, remoteVolumesDirs, vols); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func deletefromstablemap(files []string, stmap map[string]bool) []string {

	if len(files) == 0 {
		panic("files is 0")
	}
	if len(stmap) == 0 {
		log.Println("stmap is 0")
	}
	for i := 0; i < len(files); i++ {
		if stmap[files[i]] == true {
			log.Printf("delete %v\n", files[i])
			files[i] = files[len(files)-1]
			files = files[:len(files)-1]
			i--
		}
	}
	return files
}

//将打开的文件保存到/run/migration/is/openfile.jsob目录下
func SaveOpenFile(checkdir, path string, vol Volumes) error {

	files, err := getFiles(checkdir)
	if err != nil {
		log.Println(err)
		return err
	}
	data := syncNeedFiles(files, vol)
	if err != nil {
		log.Println(err)
		return err
	}

	err = dumpFiles(data, path)
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func dumpFiles(data []string, path string) error {
	if len(data) == 0 {
		log.Println("open file is nil")
		return nil
	}

	f, err := os.Create(path)
	if err != nil {
		log.Println(err)
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "	")
	for _, v := range data {
		if err = enc.Encode(v); err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
}

//获得所有打开的文件
func getFiles(path string) ([]string, error) {
	var (
		err error
		res = make([]string, 0)
	)

	args := []string{"x"}
	args = append(args, path, "fds")
	cmd := exec.Command("crit", args...)
	out, err := cmd.Output()
	if err != nil {
		log.Println(err, string(out))
		return res, err
	}
	//log.Printf("out is %v\n",string(out))

	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		txt := s.Text()
		//log.Println(s.Text())
		sp := strings.Split(txt, ":")
		//log.Printf("sp is %v,len is %v\n",sp,len(sp))
		if len(sp) == 2 {
			if sp[1][1] == '/' {
				res = append(res, sp[1][1:])
			}
		}
	}
	res = res[:len(res)-2]
	//log.Printf("res is %v,len is %v\n",res,len(res))
	return res, nil
}

//找到再数据卷中的文件
func syncNeedFiles(files []string, vol Volumes) []string {
	log.Printf("dst is %v\n", vol.dst)
	var (
		res = make([]string, 0)
	)

	if !vol.isWrite {
		panic("vol must be writeable!")
	}

	for i := 0; i < len(files); i++ {
		right := strings.TrimPrefix(files[i], vol.dst)
		if len(right) != len(files[i]) {
			//add this to res
			//log.Printf("path is %v,right is %v\n",files[i],right)
			//l:=filepath.Join(v.src,right)
			//log.Printf("copy file is %v\n",l)
			//log.Printf("right is %v\n",right)
			res = append(res, right[1:])
		}
	}

	//log.Printf("len is %v\n",len(res))
	return res
}

func fastCopy(files []string, ip string, remote string, vol Volumes) error {
	var (
		err error
		wg  sync.WaitGroup
		st  time.Time
	)
	if len(files) == 0 {
		log.Println("files len is 0")
		return nil
	}

	//log.Printf("f is %v\n",files)
	if err = os.Chdir(vol.src); err != nil {
		log.Println(err)
	}

	st = time.Now()

	for _, v := range files {
		log.Printf("fdsync:%v\n",v)
		wg.Add(1)

		go func(file string) {
			defer wg.Done()
			log.Printf("fdsync:%v\n", file)
			if err = RemoteCopyFileRsync(file, remote, ip); err != nil {
				log.Println(err)
			}
		}(v)

	}

	wg.Wait()

	log.Printf("fastcopy time is %v\n", time.Since(st))
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

	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("rsync error:%v,out:%v\n", err, string(out))
		log.Printf("cmd is %v\n", cmd.Args)
	}
	return err
}
