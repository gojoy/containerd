package supervisor

import (
	"errors"
	"fmt"
	"github.com/containerd/containerd/api/grpc/types"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/supervisor/migration"
	"github.com/sirupsen/logrus"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

//
type MigrationTask struct {
	baseTask
	TargetMachine
	Id   string
	Args []string
}

type TargetMachine struct {
	Host string
	Port uint32
}

var (
	TimeLogger *log.Logger
	TimeLogPos = "/run/migration/time.log"
)

func (s *Supervisor) StartMigration(t *MigrationTask) error {
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp:true})
	f, err := os.OpenFile(TimeLogPos, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		logrus.Printf("set timelog err:%v\n", err)
		return err
	}

	TimeLogger = log.New(f, "TimeLog:  ", log.LUTC|log.Lshortfile)

	defer f.Close()

	//c, err := t.checkContainers(s)
	//if err != nil {
	//	logrus.Println(err)
	//	return err
	//}
	//
	//if err = t.checkTargetMachine(); err != nil {
	//	return err
	//}

	c, _, err := t.PerMigrationTask(s)
	if err != nil {
		logrus.Printf("premigration faild:%v\n", err)
		return err
	}

	if err = t.startMigrationTask(c); err != nil {
		logrus.Println("start error: ", err)
		return err
	}

	logrus.Println("migration Finish")
	return nil
}

func (t *MigrationTask) PerMigrationTask(s *Supervisor) (*containerInfo, types.APIClient, error) {
	start := time.Now()
	TimeLogger.Printf("start premigration task at %v\n", start.String())

	c, err := t.checkContainers(s)
	if err != nil {
		logrus.Println(err)
		return nil, nil, err
	}

	rpclient, err := t.checkTargetMachine()
	if err != nil {
		return nil, nil, err
	}

	end := time.Now()
	TimeLogger.Printf("end premigration task at %v\n", end.String())
	TimeLogger.Printf("pre migration task cost %v\n", end.Sub(start).String())
	return c, rpclient, nil
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

func (t *MigrationTask) checkTargetMachine() (types.APIClient, error) {

	logrus.Println("check target machine")

	ip := t.Host
	port := t.Port
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, MigrationWriteErr(err.Error())
	}

	for _, addr := range addrs {
		ips := strings.SplitN(addr.String(), "/", 2)
		//fmt.Printf("network:%v,string:%v,splite:%v\n", addr.Network(), addr.String(), ips[0])
		if ips[0] == ip {
			return nil, MigrationWriteErr("Cannot Migration Localhost Machine")
		}
	}
	rpcclient, err := migration.GetClient(ip, port)
	if err != nil {
		return nil, MigrationWriteErr("cannot connect to target containerd server!" + err.Error())
	}

	return rpcclient, nil
}

func (t *MigrationTask) startMigrationTask(c *containerInfo) error {
	start := time.Now()
	TimeLogger.Printf("start migration task at %v\n", start.String())

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

	//在目的主机进行premigration准备操作
	logrus.Println("start premigration")
	if err = r.PreRemoteMigration(t.Id, l.Imagedir.GetUpperId(), t.Args); err != nil {
		logrus.Printf("premigration error: %v\n", err)
		return err
	}

	if err = r.DoRestore(); err != nil {
		return MigrationWriteErr(err.Error())
	}
	logrus.Println("done restore")

	end := time.Now()
	TimeLogger.Printf("end migration task ai %v\n", end.String())
	TimeLogger.Printf("migration task cost %v\n", end.Sub(start))

	return nil
}

func MigrationWriteErr(w string) error {
	return errors.New(fmt.Sprintf("Miration Failed:%v", w))
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
