package migration

import (
	"github.com/containerd/containerd/runtime"
	"os"
	"path/filepath"
	//"github.com/containerd/containerd/supervisor"
	"errors"
	"fmt"
)

const MigrationDir = "/run/migration"
const DumpAll = "fullDump"

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
