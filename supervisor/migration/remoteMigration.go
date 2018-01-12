package migration

import (
	//"github.com/containerd/containerd/supervisor"
	//"github.com/sirupsen/logrus"
	//"fmt"
	netcontext "golang.org/x/net/context"
	//"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/api/grpc/types"
	"path/filepath"
	"time"
	"github.com/sirupsen/logrus"
	"github.com/pkg/sftp"
)

const STDIO = "/dev/null"
const RUNTIMR  = "runc"
const LoginUser  =	"root"
const LoginPasswd  = 	"123456"
const RemoteCheckpointDir  = "/var/lib/migration/checkpoint"

//
type remoteMigration struct {
	Id string
	Rootfs string
	Bundle string
	CheckpointDir string
	ip string
	port uint32
	clienApi types.APIClient
	sftpClient *sftp.Client
}

func NewRemoteMigration(ip,id string,port uint32) (*remoteMigration,error) {
	c,err:=GetClient(ip,port)
	if err!=nil {
		return nil,err
	}
	sc,err:=GetSftpClient(LoginUser,LoginPasswd,ip,port)
	if err!=nil {
		return nil,err
	}
	r:=&remoteMigration{
		Id:id+"copy",
		ip:ip,
		port:port,
		clienApi:c,
		sftpClient:sc,
	}
	return r,nil
}

func (r *remoteMigration) DoRestore() error {
	bpath,err:=filepath.Abs(r.Bundle)
	if err!=nil {
		return nil
	}
	req:=&types.CreateContainerRequest{
		Id:r.Id,
		BundlePath:bpath,
		Checkpoint:DumpAll,
		CheckpointDir:r.CheckpointDir,
		Stdin:STDIO,
		Stdout:STDIO,
		Stderr:STDIO,
		Runtime:RUNTIMR,
	}

	if _,err=r.clienApi.CreateContainer(netcontext.Background(),req);err!=nil {
		logrus.Printf("remote restore err:%v\n",err)
		return err
	}
	time.Sleep(2*time.Second)
	if _,err=r.clienApi.UpdateProcess(netcontext.Background(),&types.UpdateProcessRequest{
		Id:r.Id,
		Pid:"Init",
		CloseStdin:true,
	});err!=nil {
		return err
	}
	return nil
}

func (r *remoteMigration) PreLoadImage(image *Image) error {

	return image.PreCopyImage(r.sftpClient)

}
//func NewRemoteMigration(t *supervisor.MigrationTask,l *localMigration) (*remoteMigration,error)  {
//	return &remoteMigration{
//		Bundle:l.Bundle,
//		CheckpointDir:l.CheckpointDir,
//		Id:t.Id+"copy",
//	},nil
//}
//
//
//func (r *remoteMigration)Dorestore(s *supervisor.Supervisor) error {
//	e := &supervisor.StartTask{}
//	e.WithContext(netcontext.Background())
//
//	e.ID=r.Id
//	e.CheckpointDir=r.CheckpointDir
//	e.Checkpoint=&runtime.Checkpoint{
//		Name:supervisor.DumpAll,
//	}
//	e.Stdin="/dev/null"
//	e.Stdout="/dev/null"
//	e.Stderr="/dev/null"
//	e.BundlePath=r.Bundle
//	e.StartResponse = make(chan supervisor.StartResponse, 1)
//	logrus.Println("restore send task")
//	s.SendTask(e)
//	if err := <-e.ErrorCh(); err != nil {
//		logrus.Println(err)
//		return  err
//	}
//	<-e.StartResponse
//
//	//fmt.Println(re.Container.Status())
//	fmt.Println("after restore")
//	return nil
//}
