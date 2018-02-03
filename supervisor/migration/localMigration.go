package migration

import (
	"github.com/containerd/containerd/runtime"
	"os"
	"path/filepath"
	//"github.com/containerd/containerd/supervisor"
	"errors"
	"fmt"
	"os/exec"
)

const MigrationDir = "/run/migration"
const DumpAll = "fullDump"
const nfsconfig=" 192.168.18.0/24(rw,async,no_root_squash,nohide)"

type localMigration struct {
	runtime.Container
	Rootfs         string
	Bundle         string
	CheckpointDir  string
	CheckpointName string
	IsDump         bool
	Imagedir          *Image
}

func NewLocalMigration(c runtime.Container) (*localMigration, error) {
	i, err := NewImage(c)
	if err != nil {
		return nil, err
	}

	l := &localMigration{}
	l.Bundle = c.Path()
	l.Container = c
	l.CheckpointDir = filepath.Join(MigrationDir, c.ID())
	l.IsDump = false
	l.Imagedir = i
	l.CheckpointName = DumpAll

	if err := os.MkdirAll(l.CheckpointDir, 0666); err != nil {
		return nil, err
	}
	return l, nil
}

//本地进行checkpoint
func (l *localMigration) DoCheckpoint() error {
	doCheckpoint := runtime.Checkpoint{
		Name:        l.CheckpointName,
		Exit:        false,
		TCP:         true,
		Shell:       true,
		UnixSockets: true,
		EmptyNS:     []string{"network"},
	}

	ldir:=filepath.Join(l.CheckpointDir,l.CheckpointName)

	if _,err:=os.Stat(ldir);err==nil {
		if err=os.RemoveAll(ldir);err!=nil {
			return err
		}
		glog.Println("checkpoint dir exist,we remove it")
	}

	return l.Checkpoint(doCheckpoint, l.CheckpointDir)
}


func (l *localMigration) DoneCheckpoint() error {
	if l.IsDump {
		return errors.New("recheckpoint")
	}
	l.IsDump = true
	return nil
}


//把本地的checkpoint文件夹拷贝到远程主机
func (l *localMigration) CopyCheckPointToRemote(r *remoteMigration) error {
	if r == nil {
		return fmt.Errorf("Err: remote nil\n ")
	}

	if err := RemoteCopyDir(l.CheckpointDir, r.CheckpointDir, r.sftpClient); err != nil {
		glog.Println(err)
		return err
	}
	return nil
}

func (l *localMigration) SetVolumeNfsMount() (bool,error) {
	var (
		count int
		err error
	)
	spec,err:=LoadSpec(l.Container)
	if err!=nil {
		return false,err
	}
	if len(spec.Mounts)==0 {
		return false, nil
	}

	//f,err:=os.Open("/etc/export")
	f,err:=os.OpenFile("/etc/exports",os.O_RDWR|os.O_APPEND,0666)
	if err!=nil {
		glog.Println(err)
		return true,err
	}
	defer f.Close()


	for _,v:=range spec.Mounts {
		if v.Type=="bind" && len(v.Options)==1 &&v.Options[0]=="rbind" {
			count++
			if _,err=fmt.Fprintf(f,"%s %s",v.Source,nfsconfig);err!=nil {
				glog.Println(err)
				return true,err
			}
		}
	}
	if count==0 {
		return false,nil
	}

	if err=FlushNfsConfig();err!=nil {
		return true,err
	}
	return true,nil
}