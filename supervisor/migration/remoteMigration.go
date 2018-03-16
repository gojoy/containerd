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

//const RemoteOverlay="/var/lib/migration/overlay/diff-id.." 远程主机的镜像层文件目录
const STDIO = "/dev/null"
const RunTime = "runc"
const LoginUser = "root"
const LoginPasswd = "123456"
const SftpPort = 22
const RemoteCheckpointDir = "/var/lib/migration/checkpoint"
const RemoteDir = "/var/lib/migration/containers"

//
type remoteMigration struct {
	Id             string
	Rootfs         string
	Bundle         string
	CheckpointDir  string
	CheckpointName string
	ip             string
	port           uint32
	clienApi       types.APIClient
	sftpClient     *sftp.Client
	spec           specs.Spec
}

func NewRemoteMigration(ip, id string, port uint32) (*remoteMigration, error) {

	logrus.Println("get grpc client")
	c, err := GetClient(ip, port)
	if err != nil {
		return nil, err
	}

	logrus.Println("get sftp client")
	sc, err := GetSftpClient(LoginUser, LoginPasswd, ip, SftpPort)
	if err != nil {
		return nil, err
	}

	r := &remoteMigration{
		Id:             id + "copy",
		ip:             ip,
		Bundle:         filepath.Join(RemoteDir, id+"copy"),
		CheckpointDir:  filepath.Join(RemoteCheckpointDir, id+"copy"),
		CheckpointName: DumpAll,
		port:           port,
		clienApi:       c,
		sftpClient:     sc,
	}
	return r, nil
}

//在远程主机进行恢复
func (r *remoteMigration) DoRestore() error {

	//just log it,do nothing
	glog.Println("Do Remote Restore")
	return nil

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
		Runtime:       RunTime,
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

func (r *remoteMigration) PreLoadImage(e chan error, image *Image) {

	//glog.Println("start precopy image")
	//err := image.PreCopyImage(r.sftpClient, r)
	//if err != nil {
	//	glog.Println(err)
	//}
	var err error = nil
	glog.Println("we do nothing,just return to main goroutine")
	e <- err

}

//在远程主机创建对应的config.json文件
func (r *remoteMigration) SetSpec(l *localMigration) error {
	if l == nil {
		return fmt.Errorf("Err: local is nil\n")
	}
	rspec, err := LoadSpec(l.Container)
	if err != nil {
		glog.Println(err)
		return err
	}
	//rspec.Root.Path = r.Rootfs
	rspec.Root.Readonly = false

	rfile := filepath.Join(r.Bundle, "config.json")

	if _, err := r.sftpClient.Stat(rfile); err != nil {
		if err == os.ErrNotExist {

			if err = RemoteMkdirAll(rfile, r.sftpClient); err != nil {
				glog.Println(err)
				return err
			}

			if specf, err := r.sftpClient.Create(rfile); err != nil {
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

	//glog.Println("Remote Has Config.json\n")
	return nil
}

//向目的主机发送grpc请求
func (r *remoteMigration) PreRemoteMigration(id, upperid string) error {

	//glog.Printf("upperid is %v\n",upperid)
	var (
		err                     error
		Id                      = id
		srcip, imagename, Cname string
		vol                     []Volumes
	)
	vol, err = GetVolume(Id)
	if err != nil {
		glog.Println(err)
		return err
	}
	imagename, err = GetImage(Id)
	if err != nil {
		glog.Println(err)
		return err
	}
	Cname, err = GetCName(Id)
	if err != nil {
		glog.Println(err)
		return err
	}
	srcip, err = GetIp()
	if err != nil {
		glog.Println(err)
		return err
	}

	preRequest := &types.PreMigrationRequest{
		Id:        Id,
		Upperid:   upperid,
		ImageName: imagename,
		SrcIp:     srcip,
		CName:     Cname,
	}
	pvol := make([]*types.Volumes, 0)
	for _, v := range vol {
		pvol = append(pvol, &types.Volumes{Dst: v.dst, Src: v.src})
	}
	preRequest.Vol = pvol

	if _, err = r.clienApi.PreMigration(netcontext.Background(), preRequest); err != nil {
		glog.Println(err)
		return err
	}
	return nil
}

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
