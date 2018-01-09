package migration

import (
	"github.com/containerd/containerd/runtime"
	"path/filepath"
	"os"
	//"github.com/containerd/containerd/supervisor"
	"errors"
)

const MigrationDir = "/run/migration"
const DumpAll  = "fullDump"

type localMigration struct {
	runtime.Container
	Rootfs string
	Bundle string
	CheckpointDir string
	IsDump bool
	image *Image
}

func NewLocalMigration(c runtime.Container) (*localMigration, error) {
	l:=&localMigration{}
	l.Bundle=c.Path()
	l.Container=c
	l.CheckpointDir=filepath.Join(MigrationDir,c.ID())
	l.IsDump=false

	if err:=os.MkdirAll(l.CheckpointDir,0666);err!=nil {
		return nil,err
	}
	return l,nil
}

func (l *localMigration) DoCheckpoint() error {
	doCheckpoint:=runtime.Checkpoint{
		Name:DumpAll,
		Exit:false,
		TCP:true,
		Shell:true,
		UnixSockets:true,
		EmptyNS:[]string{"network"},
	}
	return l.Checkpoint(doCheckpoint,l.CheckpointDir)
}


func (l *localMigration)DoneCheckpoint() error {
	if l.IsDump {
		return errors.New("recheckpoint")
	}
	l.IsDump=true
	return nil
}

func (l *localMigration)loadImage(image *Image) error {
	i,err:=NewImage(l.Container)
	if err!=nil {
		return err
	}
	l.image=i
	return nil
}