package supervisor

import (
	"errors"
	"fmt"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/supervisor/migration"
	"github.com/sirupsen/logrus"
	"net"
	"strings"
	"time"
)

//
type MigrationTask struct {
	baseTask
	TargetMachine
	Id string
}

type TargetMachine struct {
	Host string
	Port uint32
}

//
//type localMigration struct {
//	*containerInfo
//	Rootfs string
//	Bundle string
//	CheckpointDir string
//	IsDump bool
//
//}
//
//type remoteMigration struct {
//	Id string
//	Rootfs string
//	Bundle string
//	CheckpointDir string
//}

func (s *Supervisor) StartMigration(t *MigrationTask) error {
	startTime := time.Now()

	logrus.Printf("startMigration %v\n", startTime)

	c, err := t.checkContainers(s)
	if err != nil {
		logrus.Println(err)
		return err
	}

	if err = t.checkTargetMachine(); err != nil {
		return err
	}

	if err = t.startMigration(c); err != nil {
		logrus.Println("start error: ", err)
		return err
	}

	logrus.Println("migration Finish")
	return nil
}

func (t *MigrationTask) checkContainers(s *Supervisor) (*containerInfo, error) {

	logrus.Println("check containers exist")

	i, ok := s.containers[t.Id]
	if !ok {
		return nil, MigrationWriteErr(fmt.Sprintf("Container %v Not Exist\n", t.Id))
	}
	if i.container.State() != runtime.Running {
		return nil, MigrationWriteErr("Container not running")
	}
	return i, nil
}

func (t *MigrationTask) checkTargetMachine() error {

	logrus.Println("check target machine")

	ip := t.Host
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return MigrationWriteErr(err.Error())
	}
	for _, addr := range addrs {

		ips := strings.SplitN(addr.String(), "/", 2)
		//fmt.Printf("network:%v,string:%v,splite:%v\n", addr.Network(), addr.String(), ips[0])
		if ips[0] == ip {
			return MigrationWriteErr("Cannot Migration Localhost Machine")
		}
	}
	return nil
}

func MigrationWriteErr(w string) error {
	return errors.New(fmt.Sprintf("Miration Failed:%v", w))
}

//func (t *MigrationTask) startCopyImage(c *containerInfo) error {
//	image, err := migration.NewImage(c.container)
//	if err != nil {
//		return err
//	}
//	image.Path()
//	return nil
//}

func (t *MigrationTask) startMigration(c *containerInfo) error {
	var (
		e   = make(chan error)
		err error
	)

	logrus.Println("new local")
	l, err := migration.NewLocalMigration(c.container)
	if err != nil {
		return MigrationWriteErr(err.Error())
	}

	logrus.Println("new remote")
	r, err := migration.NewRemoteMigration(t.Host, t.Id, t.Port)
	if err != nil {
		return MigrationWriteErr(err.Error())
	}

	logrus.Println("start preload image in goroutine")
	go r.PreLoadImage(e, l.Imagedir)

	//TODO 将hostpath的目录nfs到远程挂载 准备在本机的工作
	logrus.Println("start nfs hostpath")
	if err = l.SetNfsExport(); err != nil {
		logrus.Println("nfs", err)
		return err
	}

	logrus.Println("set spec")
	if err = r.SetSpec(l); err != nil {
		return err
	}

	if err = <-e; err != nil {
		return MigrationWriteErr(err.Error())
	}
	logrus.Println("wait goroutines finish")

	logrus.Println("do checkpoint")
	if err = l.DoCheckpoint(); err != nil {
		return err
	}

	//TODO 远程overlay mount各个目录 开始惰迁移

	if err = l.DoneCheckpoint(); err != nil {
		return err
	}

	logrus.Println("copy checkpoint dir")
	if err = l.CopyCheckPointToRemote(r); err != nil {
		return err
	}

	logrus.Println("copy upperdir")
	if err = l.CopyUpperToRemote(r); err != nil {
		return err
	}

	//r,_:=migration.NewRemoteMigration(t,l)
	//在目的主机进行premigration准备操作
	logrus.Println("start premigration")
	if err = r.PreRemoteMigration(t.Id, l.Imagedir.GetUpperId()); err != nil {
		logrus.Printf("premigration error: %v\n", err)
		return err
	}

	if err = r.DoRestore(); err != nil {
		return MigrationWriteErr(err.Error())
	}
	logrus.Println("done restore")

	return nil
}

//
////创建本地dump的目录
//func newLocalMigration(c *containerInfo) (*localMigration, error) {
//	l:=&localMigration{}
//	l.Bundle=c.container.Path()
//	l.containerInfo=c
//	l.CheckpointDir=filepath.Join(MigrationDir,c.container.ID())
//	l.IsDump=false
//
//	if err:=os.MkdirAll(l.CheckpointDir,0666);err!=nil {
//		return nil,err
//	}
//	return l,nil
//}
//
////开始执行dump
//func (l *localMigration) checkpoint() error {
//	doCheckpoint:=runtime.Checkpoint{
//		Name:DumpAll,
//		Exit:false,
//		TCP:true,
//		Shell:true,
//		UnixSockets:true,
//		EmptyNS:[]string{"network"},
//	}
//	return l.container.Checkpoint(doCheckpoint,l.CheckpointDir)
//}
//
//func (l *localMigration)doneCheckpoint() error {
//	if l.IsDump {
//		return errors.New("recheckpoint")
//	}
//	l.IsDump=true
//	return nil
//}
//
//func newRemoteMigration(t *MigrationTask,l *localMigration) (*remoteMigration,error)  {
//	return &remoteMigration{
//		Bundle:l.Bundle,
//		CheckpointDir:l.CheckpointDir,
//		Id:t.Id+"copy",
//	},nil
//}
//
//func (r *remoteMigration)restore(s *Supervisor) error {
//	e := &StartTask{}
//	e.ctx=netcontext.Background()
//	e.ID=r.Id
//	e.CheckpointDir=r.CheckpointDir
//	e.Checkpoint=&runtime.Checkpoint{
//		Name:DumpAll,
//	}
//	e.Stdin="/dev/null"
//	e.Stdout="/dev/null"
//	e.Stderr="/dev/null"
//	e.BundlePath=r.Bundle
//	e.StartResponse = make(chan StartResponse, 1)
//	logrus.Println("restore send task")
//	s.SendTask(e)
//	if err := <-e.ErrorCh(); err != nil {
//		logrus.Println(err)
//		return  err
//	}
//		<-e.StartResponse
//
//	//fmt.Println(re.Container.Status())
//	fmt.Println("after restore")
//	return nil
//}
