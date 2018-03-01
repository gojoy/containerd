package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

	glog.Println("pre lazy replication")
	if err = p.PreLazyDir(); err != nil {
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
		if err = os.MkdirAll(filepath.Join(tpath, "lazy"), 0666); err != nil {
			glog.Println(err)
			return err
		}
		if err = os.MkdirAll(filepath.Join(tpath, "upper"), 0666); err != nil {
			glog.Println(err)
			return err
		}
		if err = os.MkdirAll(filepath.Join(tpath, "work"), 0666); err != nil {
			glog.Println(err)
			return err
		}
		if err = os.MkdirAll(filepath.Join(tpath, "merge"), 0666); err != nil {
			glog.Println(err)
			return err
		}
		if err = os.MkdirAll(filepath.Join(tpath, "nfs"), 0666); err != nil {
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

	glog.Printf("upperdir is is %v\n", tmp[0].GraphDriver.Data.UpperDir)
	dst := tmp[0].GraphDriver.Data.UpperDir

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
		glog.Printf("mount cmd is %v\n",cmd.Args)
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
		l2 := filepath.Join(RemoteGetVolume(p.Id, i), "lazy")
		u := filepath.Join(RemoteGetVolume(p.Id, i), "upper")
		w := filepath.Join(RemoteGetVolume(p.Id, i), "work")
		m := filepath.Join(RemoteGetVolume(p.Id, i), "merge")
		lower := fmt.Sprintf("-olowerdir=%s:%s", l1, l2)
		upper := fmt.Sprintf("upperdir=%s", u)
		work := fmt.Sprintf("workdir=%s", w)
		other := lower + "," + upper + "," + work

		args = append(args, other, m)

		cmd := exec.Command("mount", args...)

		if err = cmd.Run(); err != nil {
			glog.Println(err)
			return err
		}

		//glog.Println(cmd.Args)
	}
	return nil
}
