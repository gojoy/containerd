package migration

import (
	"errors"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/specs"
	"github.com/pkg/sftp"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const Driver = "overlay2"
const DriverDir = "/var/lib/docker/overlay2"

type Image struct {
	runtime.Container
	spce      specs.Spec
	bundle    string
	mountType string
	lowerRO   []string
	upperRD   string
}


// 解析overlay2镜像的lower层（只读）和upper层（读写）
func NewImage(c runtime.Container) (*Image, error) {

	spec, err := LoadSpec(c)
	if err != nil {
		return nil, err
	}
	if spec.Root.Readonly {
		return nil, errors.New("Cannot Migration Readonly Containers\n")
	}
	path := spec.Root.Path
	if !strings.Contains(path, Driver) {
		return nil, errors.New("Only Support Overlay2\n")
	}

	tmp := strings.Split(path, "/")
	imageid := tmp[len(tmp)-2]
	lower, err := GetDir(imageid)
	if err != nil {
		return nil, err
	}
	s, err := LoadSpec(c)
	if err != nil {
		return nil, err
	}

	i := &Image{}
	i.spce = *s
	i.upperRD = filepath.Join(DriverDir, imageid, "diff")
	i.lowerRO = lower
	i.Container = c
	i.bundle = c.Path()
	i.mountType = Driver
	return i, nil
}

func GetDir(imageID string) ([]string, error) {
	fp, err := os.Open(filepath.Join(DriverDir, imageID, "lower"))
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	lowerContext, err := ioutil.ReadAll(fp)
	if err != nil {
		return nil, err
	}
	nowdir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	os.Chdir(filepath.Join(DriverDir, "l"))
	res := make([]string, 0)
	lowers := strings.Split(string(lowerContext), ":")

	for _, v := range lowers {
		lpath, err := os.Readlink(filepath.Join(DriverDir, v))
		if err != nil {
			return nil, err
		}
		abs, err := filepath.Abs(lpath)
		if err != nil {
			return nil, err
		}
		res = append(res, abs)
	}
	os.Chdir(nowdir)
	return res, nil
}

func (i *Image) PreCopyImage(c *sftp.Client) error {

	for _, v := range i.lowerRO {

		remotePath, err := PathToRemote(v)
		if err != nil {
			return err
		}
		glog.Printf("v :%v,r %v\n", v, remotePath)
		_, err = c.Stat(remotePath)
		if err != nil {
			//TODO 远程不存在该文件，则传输过去
			if err == os.ErrNotExist {
				//fmt.Printf("begin copy %v to %v\n",v,remotePath)
				if err = RemoteCopyDir(v, remotePath, c); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		//glog.Printf("remote has dir,so not copy:%v\n",w.Name())

	}
	return nil
}
