package migration

import (
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/supervisor/migration/lazycopydir"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

const preVolume = "/var/lib/migration/mvolume"
const remoteUpperDir = "/var/lib/migration/overlay2"

//目标容器数据卷的路径： /var/lib/migration/mvolume/containerid/volid/{lazy,upper,work,merge,nfs}
type PreMigrationInTargetMachine struct {
	Id        string
	UpperId   string
	Cname     string
	ImageName string
	Vol       []Volumes
	SrcIp     string
}

var (
	lazyreplicator=make([]*lazycopydir.LazyReplicator,0)
)

func (p *PreMigrationInTargetMachine) StartPre() error {
	var (
		err error
	)
	glog.Println("premkdir")
	if err = p.PreMkVolDir(); err != nil {
		glog.Println(err)
		return err
	}

	glog.Println("create docker container")
	if err = p.CreateDockerContainer(); err != nil {
		glog.Println(err)
		return err
	}

	glog.Println("copy upperdir")
	if err = p.CopyUpperDir(p.UpperId); err != nil {
		glog.Println(err)
		return err
	}

	glog.Println("mount nfs")
	if err = p.MountNfs(); err != nil {
		glog.Println(err)
		return err
	}

	glog.Println("start overlay dir")
	if err = p.PreLazyDir(); err != nil {
		glog.Println(err)
		return err
	}

	glog.Println("pre lazycopy")
	if err=p.StartPreLazyCopy();err!=nil {
		glog.Println(err)
		return err
	}

	glog.Println("start docker container")
	if err=p.StartDockerContainer();err!=nil {
		glog.Println(err)
		return err
	}

	glog.Printf("now container start run! %v\n",time.Now())

	glog.Println("start lazycopy")
	if err = p.StartLazyCopy(); err != nil {
		glog.Println(err)
		return err
	}
	return nil
}

func (p *PreMigrationInTargetMachine) PreMkVolDir() error {
	var (
		err error
	)
	for i := 0; i < len(p.Vol); i++ {
		tpath := filepath.Join(preVolume, p.Id, strconv.Itoa(i))
		if err = os.MkdirAll(filepath.Join(tpath, "lazy"), 0755); err != nil {
			glog.Println(err)
			return err
		}
		if err = os.MkdirAll(filepath.Join(tpath, "upper"), 0755); err != nil {
			glog.Println(err)
			return err
		}
		if err = os.MkdirAll(filepath.Join(tpath, "work"), 0755); err != nil {
			glog.Println(err)
			return err
		}
		if err = os.MkdirAll(filepath.Join(tpath, "merge"), 0755); err != nil {
			glog.Println(err)
			return err
		}
		if err = os.MkdirAll(filepath.Join(tpath, "nfs"), 0755); err != nil {
			glog.Println(err)
			return err
		}
	}

	return nil
}

func (p *PreMigrationInTargetMachine) CreateDockerContainer() error {
	var (
		err error
	)
	args1:=append([]string{"rm"},p.Cname+"copy")
	cmd1:=exec.Command("docker",args1...)
	cmd1.Run()

	args := append([]string{"create", "-P", "--security-opt", "seccomp:unconfined",
		"-e", "MYSQL_ROOT_PASSWORD=123456", "--name"},
		p.Cname+"copy")
	//args=append(args,"-v")
	for i, v := range p.Vol {
		args = append(args, "-v", fmt.Sprintf("%s:%s",
			filepath.Join(preVolume, p.Id, strconv.Itoa(i), "merge"), v.dst))
	}
	args = append(args, p.ImageName)
	cmd := exec.Command("docker", args...)
	glog.Printf("create cmd is %v\n", cmd.Args)
	if err = cmd.Run(); err != nil {
		glog.Println(err)
		return err
	}
	return nil
}

func (p *PreMigrationInTargetMachine) StartDockerContainer() error  {
	name:=p.Cname+"copy"
	args:=[]string{"start","--checkpoint-dir"}
	args=append(args,filepath.Join(RemoteCheckpointDir,p.Id+"copy"))
	args=append(args,"--checkpoint",DumpAll,name)
	cmd:=exec.Command("docker",args...)
	glog.Printf("start docker cmd is %v\n",cmd.Args)
	if err:=cmd.Run();err!=nil {
		glog.Println(err)
		return err
	}

	return nil
}

func (p *PreMigrationInTargetMachine) CopyUpperDir(imageid string) error {
	var (
		err  error
		name = p.Cname + "copy"
		tmp  []struct {
			GraphDriver struct{ Data struct{ UpperDir string } }
		}
	)

	src := filepath.Join(remoteUpperDir, imageid, "diff")
	//glog.Printf("imageid is %v\n",imageid)
	_, err = os.Stat(src)
	if err != nil {
		glog.Printf("Remote Don't Has  Upperdir %v:", err)
		//return err
	}

	args := append([]string{"inspect"}, name)
	cmd := exec.Command("docker", args...)

	bs, err := cmd.Output()
	if err != nil {
		glog.Println(err)
		return err
	}

	if err = json.Unmarshal(bs, &tmp); err != nil {
		glog.Println(err)
		return err
	}

	dst := tmp[0].GraphDriver.Data.UpperDir

	if src[len(src)-1] != '/' {
		src = src + "/"
	}

	glog.Printf("src is is %v,dst is %v\n", src, tmp[0].GraphDriver.Data.UpperDir)

	if err = CopyDirLocal(src, dst); err != nil {
		glog.Println(err)
		return err
	}

	return nil
}

//将目标卷挂载到/var/lib/migration/mvolume/id/volid/nfs
func (p *PreMigrationInTargetMachine) MountNfs() error {
	var (
		err error
		vol = p.Vol
	)
	for i, v := range vol {
		args := append([]string{"-t", "nfs", "-o", "v3"},
			fmt.Sprintf("%s:%s", p.SrcIp, v.src))

		args = append(args, filepath.Join(RemoteGetVolume(p.Id, i), "nfs"))

		cmd := exec.Command("mount", args...)
		glog.Printf("mount cmd is %v\n", cmd.Args)
		if err = cmd.Run(); err != nil {
			glog.Println(err)
			return err
		}
	}
	return nil
}

func RemoteGetVolume(id string, volid int) string {
	return filepath.Join(preVolume, id, strconv.Itoa(volid))
}

//mount -t overlay overlay -olowerdir=nfs:lazy,upperdir=upper,workdir=work merge
func (p *PreMigrationInTargetMachine) PreLazyDir() error {

	var (
		err error
	)
	for i := 0; i < len(p.Vol); i++ {
		args := []string{"-t", "overlay", "overlay"}
		l1 := filepath.Join(RemoteGetVolume(p.Id, i), "nfs")
		//l2 := filepath.Join(RemoteGetVolume(p.Id, i), "lazy")
		u := filepath.Join(RemoteGetVolume(p.Id, i), "upper")
		w := filepath.Join(RemoteGetVolume(p.Id, i), "work")
		m := filepath.Join(RemoteGetVolume(p.Id, i), "merge")
		lower := fmt.Sprintf("-olowerdir=%s", l1)
		upper := fmt.Sprintf("upperdir=%s", u)
		work := fmt.Sprintf("workdir=%s", w)
		other := lower + "," + upper + "," + work

		args = append(args, other, m)

		cmd := exec.Command("mount", args...)

		glog.Printf("overlay cmd is %v\n", cmd.Args)
		if err = cmd.Run(); err != nil {
			glog.Println(err)
			return err
		}

		//glog.Println(cmd.Args)
	}
	return nil
}

func (p *PreMigrationInTargetMachine) StartPreLazyCopy() error {
	var (
		err                      error
		crwdir, monidir, lazydir string

	)

	for i := 0; i < len(p.Vol); i++ {
		glog.Printf("start lazy vol %d\n", i)
		crwdir = filepath.Join(RemoteGetVolume(p.Id, i), "nfs")
		monidir = filepath.Join(RemoteGetVolume(p.Id, i), "upper")
		lazydir = filepath.Join(RemoteGetVolume(p.Id, i), "lazy")
		r:=lazycopydir.NewLazyReplicator(crwdir,monidir,lazydir)
		if err=r.Prelazy();err!=nil {
			glog.Println(err)
			return err
		}
		lazyreplicator=append(lazyreplicator,r)

	}
	glog.Printf("finish pre lazy copy:%v",time.Now())
	return nil
}

func (p *PreMigrationInTargetMachine) StartLazyCopy() error {
	var (
		err error
	)
	for _,v:=range lazyreplicator {
		if err=v.Dolazycopy();err!=nil {
			glog.Println(err)
			return err
		}
	}
	return nil
}