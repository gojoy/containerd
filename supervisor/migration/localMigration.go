package migration

import (
	"github.com/containerd/containerd/runtime"
	"os"
	"path/filepath"
	"errors"
	"fmt"
	"log"
	"strconv"
)

const MigrationDir = "/run/migration"
const DumpAll = "fullDump"
//const nfsconfig = " 0.0.0.0/24(rw,async,no_root_squash,nohide)"
const nfsconfig=" *(rw,async,no_root_squash,nohide)"
const stablefile="stablefilelist.txt"
const openFileDir = "openfile.json"

type localMigration struct {
	runtime.Container
	Rootfs         string
	Bundle         string
	CheckpointDir  string
	CheckpointName string
	IsDump         bool
	Imagedir       *Image
	vols []Volumes
	basedir string
}

type Volumes struct {
	src, dst string
	isWrite bool
}

type writevol struct {
	vol Volumes
	id int
}


func NewVolumes(src, dst string,iswrite bool) Volumes {
	vol := Volumes{
		src: src,
		dst: dst,
		isWrite:iswrite,
	}
	return vol
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
	l.basedir=filepath.Join(MigrationDir,c.ID())
	if err:=os.MkdirAll(l.basedir,0666);err!=nil {
		log.Println(err)
		return nil,err
	}

	vols,err:=GetVolume(c.ID())
	if err!=nil {
		log.Println(err)
		return nil,err
	}
	l.vols=vols

	if err := os.MkdirAll(l.CheckpointDir, 0666); err != nil {
		return nil, err
	}
	return l, nil
}

//本地进行checkpoint
func (l *localMigration) DoCheckpoint() error {
	doCheckpoint := runtime.Checkpoint{
		Name:        l.CheckpointName,
		Exit:        true,
		TCP:         true,
		Shell:       true,
		UnixSockets: true,
		EmptyNS:     []string{"network"},
	}

	ldir := filepath.Join(l.CheckpointDir, l.CheckpointName)

	if _, err := os.Stat(ldir); err == nil {
		if err = os.RemoveAll(ldir); err != nil {
			return err
		}
		log.Println("checkpoint dir exist,we remove it")
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

// copy id../diff(upperdir) to remote /var/lib/migration/overlay2/id../diff
func (l *localMigration) CopyUpperToRemote(r *remoteMigration) error {

	var (
		err error
	)
	localUpperDir := l.Imagedir.upperRD
	remoteUpperDir, err := PathToRemote(localUpperDir)
	if err != nil {
		return err
	}

	if err = RemoteMkdirAll(remoteUpperDir, r.sftpClient); err != nil {
		log.Println(err)
		return err
	}

	return RemoteCopyDirRsync(localUpperDir, remoteUpperDir, r.ip)

	//return l.Imagedir.CopyUpper(r.sftpClient)

}

//copy readonly vol to remove /var/lib/migration/mvolume/id/merge
func (l *localMigration) CopyReadVolToRemote(r *remoteMigration) error {
	var (
		err error
	)
	vols,err:=GetVolume(l.ID())
	if err!=nil {
		log.Println(err)
		return err
	}
	for I,v:=range vols {
		if !v.isWrite {
			remoteVolPath:=filepath.Join(preVolume,l.ID(),strconv.Itoa(I),"merge")
			if err = RemoteMkdirAll(remoteVolPath, r.sftpClient); err != nil {
				log.Println(err)
				return err
			}
			if err=RemoteCopyDirRsync(v.src,remoteVolPath,r.ip);err!=nil {
				log.Println(err)
				return err
			}
		}
	}
	return nil
}

func (l *localMigration) CopyWriteVolToRemote(r *remoteMigration) error {
	if r==nil {
		return fmt.Errorf("Err: remote nil\n ")
	}
	vols,err:=GetVolume(l.ID())
	if err!=nil {
		log.Println(err)
		return err
	}
	for i,v:=range vols {
		if v.isWrite {
			remotePath:=filepath.Join(remoteWriteVolume,l.ID(),strconv.Itoa(i))
			if err = RemoteMkdirAll(remotePath, r.sftpClient); err != nil {
				log.Println(err)
				return err
			}
			if err=RemoteCopyDirRsync(v.src,remotePath,r.ip);err!=nil {
				log.Println(err)
				return err
			}
		}
	}
	return nil
}

//把本地的checkpoint文件夹拷贝到远程主机
func (l *localMigration) CopyCheckPointToRemote(r *remoteMigration) error {
	if r == nil {
		return fmt.Errorf("Err: remote nil\n ")
	}

	if err := RemoteMkdirAll(r.CheckpointDir, r.sftpClient); err != nil {
		log.Println(err)
		return err
	}
	if err := RemoteCopyDirRsync(l.CheckpointDir, r.CheckpointDir, r.ip); err != nil {
		log.Println(err)
		return err
	}

	//if err := RemoteCopyDir(l.CheckpointDir, r.CheckpointDir, r.sftpClient); err != nil {
	//	log.Println(err)
	//	return err
	//}
	return nil
}

func (l *localMigration) SaveOpenFile() error {
	var (
		err error
		writevols=make([]Volumes,0)
	)
	vol,err:=GetVolume(l.ID())
	if err!=nil {
		log.Println(err)
		return err
	}
	for _,v:=range vol {
		if v.isWrite {
			writevols=append(writevols,v)
		}
	}
	if len(writevols)!=1 {
		log.Println("write vols nil")
		return errors.New("write vols nil")
	}
	path:=filepath.Join(l.basedir,openFileDir)
	ckdir:=filepath.Join(l.CheckpointDir,l.CheckpointName)
	err=SaveOpenFile(ckdir,path,writevols[0])
	if err!=nil {
		log.Println(err)
		return err
	}
	return nil
}

//需要重写，通过docker inspect 获取数据卷
//func (l *localMigration) SetVolumeNfsMount() (bool, error) {
//	var (
//		count int
//		err   error
//	)
//	spec, err := LoadSpec(l.Container)
//	if err != nil {
//		return false, err
//	}
//	if len(spec.Mounts) == 0 {
//		log.Println(spec.Mounts)
//		return false, errors.New("nfs error: no mounts\n")
//	}
//
//	//f,err:=os.Open("/etc/export")
//	f, err := os.OpenFile("/etc/exports", os.O_RDWR|os.O_APPEND, 0666)
//	if err != nil {
//		log.Println(err)
//		return true, err
//	}
//	defer f.Close()
//
//	for _, v := range spec.Mounts {
//		log.Printf("type is %v,optionsis %v,dst is %v\n", v.Type, v.Options, v.Destination)
//		if v.Type == "bind" && len(v.Options) == 1 && v.Options[0] == "rbind" {
//			count++
//			if _, err = fmt.Fprintf(f, "%s %s", v.Source, nfsconfig); err != nil {
//				log.Println(err)
//				return true, err
//			}
//		}
//	}
//	if count == 0 {
//		return false, nil
//	}
//
//	if err = FlushNfsConfig(); err != nil {
//		return true, err
//	}
//	return true, nil
//}

func (l *localMigration) SetNfsExport() error {

	vol, err := GetVolume(l.ID())
	if err != nil {
		log.Println(err)
		return err
	}
	if len(vol) == 0 {
		return nil
	}
	if err = SetNfsExport(vol); err != nil {
		log.Println(err)

	}
	return err
}


func (l *localMigration) SyncWriteFd(r *remoteMigration) error  {
	var (
		err error
		wv=make([]writevol,0)
	)
	vols,err:=GetVolume(l.ID())
	if err!=nil {
		log.Println(err)
		return err
	}
	for i,v:=range vols {
		if v.isWrite {
			wv=append(wv, struct {
				vol Volumes
				id  int
			}{vol: v, id:i })
		}
	}
	if len(wv)!=1 {
		log.Println("write vols nil")
		return errors.New("write vols nil")
	}
	rpath:=filepath.Join(remoteWriteVolume,l.ID(),strconv.Itoa(wv[0].id))
	lc:=filepath.Join(l.CheckpointDir,l.CheckpointName)
	log.Printf("start fdssync vol:%v",wv[0].vol)
	err=fdsSync(lc,rpath,wv[0].vol,r.ip)
	if err!=nil {
		log.Println(err)
		return err
	}
	log.Println("fdsync finish!")
	return nil
}

func (l *localMigration) Watchwritevol() (*volwatcher,error) {
	var (
		err error
		vwatcher *volwatcher
	)
	vwatcher=Newvolwatcher(l.vols)
	err=vwatcher.StartWatch()
	if err!=nil {
		log.Println(err)
		return nil,err
	}
	return vwatcher,nil
}

func (l *localMigration) Getstablefiles(v *volwatcher) error {
	res,err:=v.GetStablefile()
	if err!=nil {
		log.Println(err)
		return err
	}
	path:=filepath.Join(l.basedir,stablefile)
	if err=dumpFiles(res,path);err!=nil {
		log.Println(err)
		return err
	}
	return nil
}

func (l *localMigration) GetContainerMem() (uint64, error) {
	s,err:=l.Stats()
	if err!=nil {
		log.Println(err)
		return 0,err
	}
	log.Printf("mem is %v\n",s.Memory)
	return s.Memory.Usage.Usage,nil
}