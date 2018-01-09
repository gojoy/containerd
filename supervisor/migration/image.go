package migration


import (
	"github.com/containerd/containerd/runtime"
	"errors"
	"strings"
	"path/filepath"
	"os"
	"io/ioutil"
	"fmt"
	"github.com/containerd/containerd/specs"
)


const Driver 	=	"overlay2"
const DriverDir	=	"/var/lib/docker/overlay2"


type Image struct {
	runtime.Container
	spce specs.Spec
	bundle string
	mountType string
	lowerRO []string
	upperRD string
}

func NewImage(c runtime.Container) (*Image,error) {

	spec,err:=LoadSpec(c)
	if err!=nil {
		return nil,err
	}
	if spec.Root.Readonly {
		return nil,errors.New("Cannot Migration Readonly Containers\n")
	}
	path:=spec.Root.Path
	if !strings.Contains(path,Driver) {
		return nil,errors.New("Only Support Overlay2\n")
	}

	tmp:=strings.Split(path,"/")
	imageid:=tmp[len(tmp)-2]
	lower,err:=GetDir(imageid)
	if err!=nil {
		return nil,err
	}
	s,err:=LoadSpec(c)
	if err!=nil {
		return nil,err
	}

	i:=&Image{}
	i.spce=*s
	i.upperRD=filepath.Join(DriverDir,imageid,"diff")
	i.lowerRO=lower
	i.Container=c
	i.bundle=c.Path()
	i.mountType=Driver
	return i,nil
}


func GetDir(imageID string) ([]string, error) {
	fp,err:=os.Open(filepath.Join(DriverDir,imageID,"lower"))
	if err!=nil {
		return nil,err
	}
	defer fp.Close()

	lowerContext,err:=ioutil.ReadAll(fp)
	if err!=nil {
		return nil,err
	}
	nowdir,err:=os.Getwd()
	if err!=nil {
		return nil,err
	}
	os.Chdir(filepath.Join(DriverDir,"l"))
	res:=make([]string,0)
	lowers:=strings.Split(string(lowerContext),":")

	for _,v:=range lowers {
		fmt.Printf("v is %v\n",v)
		lpath,err:=os.Readlink(filepath.Join(DriverDir,v))
		if err!=nil {
			return nil,err
		}
		abs,err:=filepath.Abs(lpath)
		if err!=nil {
			return nil,err
		}
		res=append(res,abs)
	}
	os.Chdir(nowdir)
	return res,nil
}

func (i *Image) PreCopyImage(r *remoteMigration) error {

	return nil
}


