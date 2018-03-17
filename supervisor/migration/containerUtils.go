package migration

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/containerd/containerd/api/grpc/types"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/specs"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

//var (
//	log *log.Logger
//)
//
//func init() {
//	log = log.New(os.Stderr, "", log.Lshortfile)
//}

func LoadSpec(c runtime.Container) (*specs.Spec, error) {
	var spec specs.Spec
	f, err := os.Open(filepath.Join(c.Path(), "config.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := json.NewDecoder(f).Decode(&spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func GetClient(ip string, port uint32) (types.APIClient, error) {
	bindSpec := fmt.Sprintf("tcp://%v:%d", ip, port)
	grpclog.SetLogger(log.New(ioutil.Discard, "", log.LstdFlags))
	dialOpts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithTimeout(1 * time.Second)}
	dialOpts = append(dialOpts,
		grpc.WithDialer(func(s string, duration time.Duration) (net.Conn, error) {
			return net.DialTimeout("tcp", fmt.Sprintf("%v:%d", ip, port), duration)
		},
		))
	conn, err := grpc.Dial(bindSpec, dialOpts...)
	if err != nil {
		return nil, err
	}
	return types.NewAPIClient(conn), nil

}

func GetSftpClient(user, passwd, host string, port uint32) (*sftp.Client, error) {

	auth := make([]ssh.AuthMethod, 0)
	auth = append(auth, ssh.Password(passwd))
	addrConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            auth,
		Timeout:         1 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	addr := fmt.Sprintf("%s:%d", host, port)

	sshClient, err := ssh.Dial("tcp", addr, addrConfig)
	if err != nil {
		return nil, err
	}

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}

	return sftpClient, nil
}

//通过本地目录得到远程目录 把目录路径的docker变为migration
func PathToRemote(s string) (string, error) {
	if len(s) < 15 {
		return "", errors.New("local Path illegal\n")
	}
	ss := []byte(s)
	head := ss[:9]
	tail := ss[15:]
	res := string(head) + "migration" + string(tail)
	//fmt.Println("res is:",res)
	return res, nil

}

//传输本地文件夹到远程
//todo  使用rsync传输 不用sftp
func RemoteCopyDir(localPath, remotePath string, c *sftp.Client) error {
	var (
		err error
	)

	if _, err = c.Stat(remotePath); err == nil {
		log.Printf("remote has %v,we do not copy it", remotePath)
		return nil
	}

	if err = RemoteMkdirAll(remotePath, c); err != nil {
		return err
	}

	if err = os.Chdir(localPath); err != nil {
		return err
	}
	buf := make([]byte, 512)

	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			panic(err)
			return nil
		}

		if info.IsDir() {
			rpath := filepath.Join(remotePath, path)

			if err = c.Mkdir(rpath); err != nil {
				//panic(err)
				log.Println(err)
				return err
			}

		} else {

			dstf, err := c.Create(filepath.Join(remotePath, path))
			if err != nil {
				return err
			}
			defer dstf.Close()
			srcf, err := os.Open(filepath.Join(localPath, path))
			if err != nil {
				return err
			}
			defer srcf.Close()

			_, err = io.CopyBuffer(dstf, srcf, buf)
			if err != nil {
				//panic(err)
				log.Println(err)
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func RemoteCopyDirRsync(local, remote string, ip string) error {

	var (
		err error
	)
	if local[len(local)-1] != '/' {
		local = local + "/"
	}
	if remote[len(remote)-1] != '/' {
		remote = remote + "/"
	}

	args := append([]string{"-azv"}, local, "root@"+ip+":"+remote)
	//log.Printf("l is %v,r is %v,args is %v\n",local,remote,args)

	cmd := exec.Command("rsync", args...)
	//log.Printf("cmd is %v\n",cmd)
	if err = cmd.Run(); err != nil {
		log.Printf("rsync error:%v\n", err)
		log.Printf("cmd is %v\n", cmd.Args)
	}
	return err
}

func CopyDirLocal(src, dst string) error {
	var (
		err    error
		local  = src
		remote = dst
	)
	args := append([]string{"-azv"}, local, remote)

	cmd := exec.Command("rsync", args...)
	//log.Printf("cmd is %v\n",cmd)
	if err = cmd.Run(); err != nil {
		log.Printf("rsync error:%v\n", err)
		log.Printf("cmd is %v\n", cmd.Args)
	}
	return err

}

//创建所有的父文件夹，便于后续的传输
func RemoteMkdirAll(rpath string, c *sftp.Client) error {
	ps := strings.SplitAfter(rpath, "/")
	root := ""
	for _, v := range ps[:len(ps)-1] {

		root = root + v

		if _, err := c.Stat(root); err != nil {
			if err == os.ErrNotExist {
				//log.Printf("dir %v not exist,we create it\n",root)
				if err := c.Mkdir(root); err != nil {
					return err
				}
			} else {
				log.Println(err)
				return err
			}
		}
		//fmt.Println(root)
	}
	return nil
}

func FlushNfsConfig() error {
	cmd := exec.Command("exportfs", "-r")
	return cmd.Run()
}

func GetVolume(id string) ([]Volumes, error) {

	var vl []struct{ HostConfig struct{ Binds []string } }
	var res []Volumes
	//args:=append([]string{"inspect","-f","{{.HostConfig.Binds}}"},id)
	args := append([]string{"inspect"}, id)
	cmd := exec.Command("docker", args...)

	bs, err := cmd.Output()
	if err != nil {
		log.Println(err)
		return nil, err
	}

	if err = json.Unmarshal(bs, &vl); err != nil {
		log.Println(err)
		return nil, err
	}

	if len(vl) != 1 {
		log.Println("len !=1 ")
		return nil, errors.New("inspect not one\n")
	}

	for _, v := range vl[0].HostConfig.Binds {
		sp := strings.Split(v, ":")
		if len(sp) != 2 {
			log.Println("splite false")
			log.Println(sp)
			return nil, errors.New("split error\n")
		}
		res = append(res, struct{ src, dst string }{src: sp[0], dst: sp[1]})
	}

	return res, nil
}

func GetCName(id string) (string, error) {
	var (
		err error
		tmp []struct{ Name string }
	)
	args := append([]string{"inspect"}, id)
	cmd := exec.Command("docker", args...)
	bs, err := cmd.Output()
	if err != nil {
		log.Println(err)
		return "", err
	}

	if err = json.Unmarshal(bs, &tmp); err != nil {
		log.Println(err)
		return "", err
	}
	name := tmp[0].Name
	if name[0] == '/' {
		res := []byte(name)
		res = res[1:]
		return string(res), nil
	}

	return tmp[0].Name, nil
}

func GetImage(id string) (string, error) {
	var (
		res string
		err error
		tmp []struct{ Config struct{ Image string } }
	)
	args := append([]string{"inspect"}, id)
	cmd := exec.Command("docker", args...)

	bs, err := cmd.Output()
	if err != nil {
		log.Println(err)
		return res, err
	}

	if err = json.Unmarshal(bs, &tmp); err != nil {
		log.Println(err)
		return res, err
	}

	res = tmp[0].Config.Image

	return res, err
}

func SetNfsExport(vol []Volumes) error {

	f, err := os.OpenFile("/etc/exports", os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		log.Println(err)
		return err
	}
	defer f.Close()

	for _, v := range vol {
		if _, err = fmt.Fprintf(f, "%s %s\n", v.src, nfsconfig); err != nil {
			log.Println(err)
			return err
		}
	}

	f.Sync()

	if err = FlushNfsConfig(); err != nil {
		log.Println(err)
		return err
	}
	return nil

}

func GetIp() (string, error) {
	var (
		err error
	)
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Println(err)
		return "", err
	}
	for _, address := range addrs {

		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				//log.Printf("ip is %v\n", ipnet.IP.String())
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", errors.New("Cannot Get Ip\n")
}
