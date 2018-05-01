package migration

import (
	"errors"
	"github.com/containerd/containerd/runtime"
	"github.com/containerd/containerd/specs"
	"github.com/pkg/sftp"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"context"
)

const Driver = "overlay2"
const DriverDir = "/var/lib/docker/overlay2"

type Image struct {
	runtime.Container
	spce      specs.Spec
	bundle    string
	mountType string
	// /var/lib/docker/overlay/id..
	lowerRO []string
	// /var/lib/docker/overlay/id../diff
	upperRD string
	upperid string
}

type volwatcher struct {
	vol []Volumes
	res motifyvols
	ctx context.Context
	cancel context.CancelFunc
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

	//通过root.path得到merge目录,/var/lib/docker/overlay2/imageid/merge
	// 再读取其中的lower即可得到lower目录
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

	i.upperid = imageid
	i.upperRD = filepath.Join(DriverDir, imageid, "diff")
	i.lowerRO = lower
	i.Container = c
	i.bundle = c.Path()
	i.mountType = Driver
	return i, nil
}

//lower目录为/var/lib/docker/ovaerlay2/id 不带diff
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
		//去掉路径最后的/diff
		res = append(res, abs[:len(abs)-5])
	}
	os.Chdir(nowdir)
	return res, nil
}

//把/var/lib/docker/overlay2/id/{diff(upperdir),lower,link,work}拷贝到目的主机，
// 不拷贝merge文件夹
/*
func (i *Image) CopyUpper(c *sftp.Client) error {
	//得到id/的文件夹
	mudir := i.upperRD[:len(i.upperRD)-5]
	remoteDir, err := PathToRemote(mudir)
	if err != nil {
		log.Println(err)
		return err
	}

	log.Printf("copy upper dir %v\n", mudir)

	if err = RemoteCopyDir(filepath.Join(mudir, "diff"), filepath.Join(remoteDir, "diff"), c); err != nil {
		log.Println(err)
		return err
	}
	if err = RemoteCopyDir(filepath.Join(mudir, "lower"), filepath.Join(remoteDir, "lower"), c); err != nil {
		log.Println(err)
		return err
	}
	return nil
}
*/

func (i *Image) PreCopyImage(c *sftp.Client, r *remoteMigration) error {

	for _, v := range i.lowerRO {

		remotePath, err := PathToRemote(v)
		if err != nil {
			return err
		}
		//log.Printf("v :%v,r %v\n", v, remotePath)
		_, err = c.Stat(remotePath)
		if err != nil {
			//TODO 远程不存在该文件，则传输过去
			if err == os.ErrNotExist {
				//fmt.Printf("begin copy %v to %v\n",v,remotePath)
				if err = RemoteCopyDirRsync(v, remotePath, r.ip); err != nil {
					log.Println(err)
					return err
				}
				//if err = RemoteCopyDir(v, remotePath, c); err != nil {
				//	return err
				//}
			} else {
				return err
			}
		}
		//log.Printf("remote has dir,so not copy:%v\n",w.Name())

	}
	return nil
}

func (i *Image) GetUpperId() string {
	return i.upperid
}

func Newvolwatcher(vol []Volumes) *volwatcher  {
	r:=make(motifyvols,0)
	ctx,cancel:=context.WithCancel(context.Background())
	vwatcher:=&volwatcher{
		vol:vol,
		res:r,
		ctx:ctx,
		cancel:cancel,
	}
	return vwatcher
}

func (vw *volwatcher) StartWatch() error {
	v1:=make([]Volumes,0)
	for _,v:=range vw.vol {
		if v.isWrite {
			v1=append(v1,v)
		}
	}
	if len(v1)!=1 {
		log.Println("write vol is not one")
		return errors.New("write vol is not one")
	}
	go func() {
		err:=GetMotifyFiles(v1[0].src,vw.ctx,vw.res)
		if err!=nil {
			panic(err)
		}
	}()
	return nil
}

func (vw *volwatcher) StopWatch()  {
	vw.cancel()
}

func (vw *volwatcher) GetRes() (motifyvols)  {
	return vw.res
}