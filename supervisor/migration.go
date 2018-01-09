package supervisor

import (
	"time"
	"github.com/sirupsen/logrus"
	"errors"
	"fmt"
	"github.com/containerd/containerd/runtime"
	"net"
	"strings"
	"github.com/containerd/containerd/supervisor/migration"
)

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
	startTime:=time.Now()
	fmt.Println("begin Migration")
	logrus.Printf("startMigration %v\n",startTime)
	c,err:=t.checkContainers(s)
	if err!=nil {
		fmt.Println(err)
		return err
	}

	if err=t.checkTargetMachine(s);err!=nil {
		return err
	}

	if err=t.startMigration(c,s);err!=nil {
		return err
	}
	//logrus.Printf("container %v\n",i.container.ID())
	fmt.Println("migration Finish")
	return nil
}

func (t *MigrationTask)checkContainers(s *Supervisor) (*containerInfo,error) {
	i,ok:=s.containers[t.Id]
	if !ok {
		return nil,MigrationWriteErr(fmt.Sprintf("Container %v Not Exist\n",t.Id))
	}
	if i.container.State()!=runtime.Running {
		return nil,MigrationWriteErr("Container not running")
	}
	return i,nil
}

func (t *MigrationTask) checkTargetMachine(s *Supervisor) error {
	ip:=t.Host
	addrs,err:=net.InterfaceAddrs()
	if err!=nil {
		return MigrationWriteErr(err.Error())
	}
	for _,addr:=range addrs {

		ips:=strings.SplitN(addr.String(),"/",2)
		fmt.Printf("network:%v,string:%v,splite:%v\n",addr.Network(),addr.String(),ips[0])
		if ips[0]==ip {
			return MigrationWriteErr("Cannot Migration Localhost Machine")
		}
	}
	return nil
}

func MigrationWriteErr(w string) error {
	return errors.New(fmt.Sprintf("miration failed:%v",w))
}

func (t *MigrationTask) startCopyImage(c *containerInfo) error  {
	image,err:=migration.NewImage(c.container)
	if err!=nil {
		return err
	}
	image.Path()
	return nil
}

func (t *MigrationTask) startMigration(c *containerInfo,s *Supervisor) error {
	l,err:=migration.NewLocalMigration(c.container)
	if err!=nil {
		return err
	}
	if err=l.DoCheckpoint();err!=nil {
		return err
	}
	if err=l.DoneCheckpoint();err!=nil {
		return err
	}

	//r,_:=migration.NewRemoteMigration(t,l)

	logrus.Println("begin restore")
	//go r.Dorestore(s)
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