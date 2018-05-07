package supervisor

import (
	"errors"
	"fmt"
	"github.com/containerd/containerd/api/grpc/types"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/supervisor/migration"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

const (
	TimeLogPos  = "/run/migration/time.log"
	TimeLogPath = "/run/migration"
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

type doubleLoger struct {
	w io.Writer
}

func (d *doubleLoger) Write(p []byte) (n int, err error) {
	os.Stderr.Write(p)
	return d.w.Write(p)
}

func NewDoubleLoger(f io.Writer) io.Writer {
	return &doubleLoger{w: f}
}

func init() {

	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
	log.SetPrefix("Migration log:")
	os.MkdirAll(TimeLogPath, 0755)
	f, err := os.OpenFile(TimeLogPos, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("set timelog err:%v\n", err)
		panic(err)
	}
	//log.SetOutput(NewDoubleLoger(f))
	log.SetOutput(NewDoubleLoger(f))
}

func (s *Supervisor) StartMigration(t *MigrationTask) error {

	c, _, err := t.PerMigrationTask(s)
	if err != nil {
		log.Printf("premigration faild:%v\n", err)
		return err
	}

	if err = t.startMigrationTask(c); err != nil {
		log.Println("start error: ", err)
		return err
	}

	log.Println("migration Finish")
	log.Println("~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~")
	return nil
}

func (t *MigrationTask) PerMigrationTask(s *Supervisor) (*containerInfo, types.APIClient, error) {
	start := time.Now()
	log.Printf("start premigration task at %v\n", start.String())

	c, err := t.checkContainers(s)
	if err != nil {
		log.Println(err)
		return nil, nil, err
	}

	rpclient, err := t.checkTargetMachine()
	if err != nil {
		return nil, nil, err
	}

	end := time.Now()
	log.Printf("end premigration task at %v\n", end.String())
	log.Printf("pre migration task cost %v\n", end.Sub(start).String())
	return c, rpclient, nil
}

func (t *MigrationTask) checkContainers(s *Supervisor) (*containerInfo, error) {

	log.Println("check containers exist")

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

	log.Println("check target machine")

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
	var (
		e   = make(chan error)
		err error
		st time.Time
	)
	start := time.Now()
	log.Printf("start migration task at %v\n", start.String())



	log.Println("new local")
	l, err := migration.NewLocalMigration(c.container)
	if err != nil {
		log.Println(err)
		return MigrationWriteErr(err.Error())
	}

	log.Println("test getmem!")
	mem,err:=l.GetContainerMem()
	if err!=nil {
		log.Println(err)
		return err
	}
	log.Printf("mem is %v\n",mem)
	//panic("test finish!")

	log.Println("new remote")
	r, err := migration.NewRemoteMigration(t.Host, t.Id, t.Port)
	if err != nil {
		log.Println(err)
		return MigrationWriteErr(err.Error())
	}

	log.Println("start preload image in goroutine")
	go r.PreLoadImage(e, l.Imagedir)

	log.Println("copy readonly to remote")
	if err=l.CopyReadVolToRemote(r);err!=nil {
		log.Println(err)
		return err
	}

	log.Println("start watch write vol")
	vwather,err:=l.Watchwritevol()
	if err!=nil {
		log.Println(err)
		return err
	}

	log.Println("copy write vol to remote")
	if err=l.CopyWriteVolToRemote(r);err!=nil {
		log.Println(err)
		return err
	}

	log.Println("start precopy mem")
	time.Sleep(10*time.Second)

	//TODO 将hostpath的目录nfs到远程挂载 准备在本机的工作
	log.Println("start nfs hostpath")
	if err = l.SetNfsExport(); err != nil {
		log.Println("nfs", err)
		return err
	}

	log.Println("set spec")
	if err = r.SetSpec(l); err != nil {
		return err
	}

	if err = <-e; err != nil {
		return MigrationWriteErr(err.Error())
	}
	log.Println("wait goroutines finish")

	log.Println("do checkpoint")
	if err = l.DoCheckpoint(); err != nil {
		return err
	}

	//TODO 远程overlay mount各个目录 开始惰迁移

	if err = l.DoneCheckpoint(); err != nil {
		return err
	}

	log.Println("get stable filelist")
	stablemap,err:=l.Getstablefiles(vwather)
	if err!=nil {
		log.Println(err)
		return err
	}

	log.Println("save openfile json")
	if err=l.SaveOpenFile();err!=nil {
		log.Println(err)
		return err
	}

	log.Println("fdsync files!")
	st=time.Now()
	if err=l.SyncWriteFd(r,stablemap);err!=nil {
		log.Println(err)
		return err
	}
	log.Printf("fdsync time:%v\n",time.Since(st))

	log.Println("directrsync!")
	st=time.Now()
	if err=l.DirectRsync(r);err!=nil {
		log.Println(err)
		return err
	}
	log.Printf("direct time:%v\n",time.Since(st))

	log.Println("copy checkpoint dir")
	if err = l.CopyCheckPointToRemote(r); err != nil {
		return err
	}

	log.Println("copy upperdir")
	if err = l.CopyUpperToRemote(r); err != nil {
		return err
	}

	//在目的主机进行premigration准备操作
	log.Println("start premigration")
	if err = r.PreRemoteMigration(t.Id, l.Imagedir.GetUpperId(), t.Args); err != nil {
		log.Printf("premigration error: %v\n", err)
		return err
	}

	if err = r.DoRestore(); err != nil {
		return MigrationWriteErr(err.Error())
	}
	log.Println("done restore")

	end := time.Now()
	log.Printf("end migration task ai %v\n", end.String())
	log.Printf("migration task cost %v\n", end.Sub(start))

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
//	log.Println("restore send task")
//	s.SendTask(e)
//	if err := <-e.ErrorCh(); err != nil {
//		log.Println(err)
//		return  err
//	}
//		<-e.StartResponse
//
//	//fmt.Println(re.Container.Status())
//	fmt.Println("after restore")
//	return nil
//}
