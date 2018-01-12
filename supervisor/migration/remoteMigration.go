package migration

import (
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/api/grpc/types"
	"github.com/containerd/containerd/specs"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	netcontext "golang.org/x/net/context"
	"os"
	"path/filepath"
	"time"

)

const STDIO = "/dev/null"
const RUNTIMR = "runc"
const LoginUser = "root"
const LoginPasswd = "123456"
const RemoteCheckpointDir = "/var/lib/migration/checkpoint"

//
type remoteMigration struct {
	Id            string
	Rootfs        string
	Bundle        string
	CheckpointDir string
	CheckpointName string
	ip            string
	port          uint32
	clienApi      types.APIClient
	sftpClient    *sftp.Client
	spec          specs.Spec
}

func NewRemoteMigration(ip, id string, port uint32) (*remoteMigration, error) {
	c, err := GetClient(ip, port)
	if err != nil {
		return nil, err
	}
	sc, err := GetSftpClient(LoginUser, LoginPasswd, ip, port)
	if err != nil {
		return nil, err
	}
	r := &remoteMigration{
		Id:            id + "copy",
		ip:            ip,
		CheckpointDir: RemoteCheckpointDir,
		CheckpointName:DumpAll,
		port:          port,
		clienApi:      c,
		sftpClient:    sc,
	}
	return r, nil
}

//在远程主机进行恢复
func (r *remoteMigration) DoRestore() error {
	bpath, err := filepath.Abs(r.Bundle)
	if err != nil {
		return nil
	}
	req := &types.CreateContainerRequest{
		Id:            r.Id,
		BundlePath:    bpath,
		Checkpoint:    DumpAll,
		CheckpointDir: r.CheckpointDir,
		Stdin:         STDIO,
		Stdout:        STDIO,
		Stderr:        STDIO,
		Runtime:       RUNTIMR,
	}

	//runc create
	if _, err = r.clienApi.CreateContainer(netcontext.Background(), req); err != nil {
		logrus.Printf("remote restore err:%v\n", err)
		return err
	}
	time.Sleep(2 * time.Second)
	//runc start
	if _, err = r.clienApi.UpdateProcess(netcontext.Background(), &types.UpdateProcessRequest{
		Id:         r.Id,
		Pid:        "Init",
		CloseStdin: true,
	}); err != nil {
		return err
	}
	return nil
}

func (r *remoteMigration) PreLoadImage(e chan error,image *Image)  {

	err:=image.PreCopyImage(r.sftpClient)
	if err!=nil {
		glog.Println(err)
	}
	e<-err


}

//在远程主机创建对应的config.json文件
func (r *remoteMigration) setSpec(l *localMigration) error {
	if l == nil {
		return fmt.Errorf("Err: local is nil\n")
	}
	rspec, err := LoadSpec(l.Container)
	if err != nil {
		glog.Println(err)
		return err
	}
	rspec.Root.Path = r.Rootfs
	rspec.Root.Readonly = false

	if _, err := r.sftpClient.Stat(filepath.Join(r.Bundle, "config.json")); err != nil {
		if err == os.ErrNotExist {
			if specf, err := r.sftpClient.Create(filepath.Join(r.Bundle, "config.json")); err != nil {
				glog.Println(err)
				return err
			} else {
				if err = json.NewEncoder(specf).Encode(rspec); err != nil {
					glog.Println(err)
					return err
				}
			}

		} else {
			glog.Println(err)
			return err
		}
	}
	return fmt.Errorf("Remote Has Config.json\n")
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
