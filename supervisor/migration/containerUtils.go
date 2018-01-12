package migration

import (
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/specs"
	"os"
	"path/filepath"
	"encoding/json"
	"google.golang.org/grpc"
	"github.com/containerd/containerd/api/grpc/types"
	"google.golang.org/grpc/grpclog"
	"io/ioutil"
	"log"
	"time"
	"net"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"errors"
	"io"
	"strings"
)

var (
	glog *log.Logger
)

func init() {
	glog=log.New(os.Stderr,"",log.Lshortfile)
}

func LoadSpec(c runtime.Container) (*specs.Spec,error) {
	var spec specs.Spec
	f,err:=os.Open(filepath.Join(c.Path(),"config.json"))
	if err!=nil {
		return nil,err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&spec); err != nil {
		return nil, err
	}
	return &spec,nil
}

func GetClient(ip string,port uint32) (types.APIClient,error)  {
	bindSpec:=fmt.Sprintf("tcp://%v:%d",ip,port)
	grpclog.SetLogger(log.New(ioutil.Discard, "", log.LstdFlags))
	dialOpts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithTimeout(1*time.Second)}
	dialOpts=append(dialOpts,
		grpc.WithDialer(func(s string, duration time.Duration) (net.Conn, error) {
			return net.DialTimeout("tcp",fmt.Sprintf("%v:%d",ip,port),duration)
		},
		))
	conn, err := grpc.Dial(bindSpec, dialOpts...)
	if err!=nil {
		return nil,err
	}
	return types.NewAPIClient(conn),nil

}


func GetSftpClient(user,passwd,host string,port uint32) (*sftp.Client,error) {

	auth:=make([]ssh.AuthMethod,0)
	auth=append(auth,ssh.Password(passwd))
	addrConfig:=&ssh.ClientConfig{
		User:user,
		Auth:auth,
		Timeout:10*time.Second,
		HostKeyCallback:ssh.InsecureIgnoreHostKey(),
	}
	addr:=fmt.Sprintf("%s:%d",host,port)

	sshClient,err:=ssh.Dial("tcp",addr,addrConfig)
	if err!=nil {
		return nil,err
	}


	sftpClient,err:=sftp.NewClient(sshClient)
	if err!=nil {
		return nil,err
	}

	return sftpClient,nil
}

//通过本地目录得到远程目录 把目录路径的docker变为migration
func PathToRemote(s string) (string, error) {
	if len(s)<15 {
		return "",errors.New("local Path illegal\n")
	}
	ss:=[]byte(s)
	head:=ss[:9]
	tail:=ss[15:]
	res:=string(head)+"migration"+string(tail)
	//fmt.Println("res is:",res)
	return res,nil

}

//传输本地文件夹到远程
func RemoteCopyDir(localPath, remotePath string, c *sftp.Client) error {
	var (
		err error
	)

	if err=RemoteMkdirAll(remotePath,c);err!=nil {
		return err
	}

	if err=os.Chdir(localPath);err!=nil {
		return err
	}
	buf:=make([]byte,512)

	err=filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err!=nil {
			panic(err)
			return nil
		}

		if info.IsDir() {
			rpath:=filepath.Join(remotePath,path)

			if err=c.Mkdir(rpath);err!=nil {
				//panic(err)
				glog.Println(err)
				return err
			}
			
		} else {

			dstf,err:=c.Create(filepath.Join(remotePath,path))
			if err!=nil {
				return err
			}
			defer dstf.Close()
			srcf,err:=os.Open(filepath.Join(localPath,path))
			if err!=nil {
				return err
			}
			defer srcf.Close()

			_,err=io.CopyBuffer(dstf,srcf,buf)
			if err!=nil {
				//panic(err)
				glog.Println(err)
				return err
			}
		}
		return nil
	})
	if err!=nil {
		glog.Println(err)
		return err
	}
	return nil
}

//创建所有的父文件夹，便于后续的传输
func RemoteMkdirAll(rpath string, c *sftp.Client) error {
	ps:=strings.SplitAfter(rpath,"/")
	root:=""
	for _,v:=range ps[:len(ps)-1] {

		root=root+v

		if _,err:=c.Stat(root);err!=nil {
			if err==os.ErrNotExist {
				//glog.Printf("dir %v not exist,we create it\n",root)
				if err:=c.Mkdir(root);err!=nil {
					return err
				}
			} else {
				glog.Println(err)
				return err
			}
		}
		//fmt.Println(root)
	}
	return nil
}
