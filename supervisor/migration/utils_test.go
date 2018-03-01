package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

var (
	id1       = "895fcefdab76736c786ed6f327080eac57a0048cb89ce949a86b8293c9bbc939"
	id        = "0a4e9597c1741c1ae755beda85461030ca87aed304292a7993f76ec4fe2a75fe"
	testpath  = "/var/lib/docker/overlay2/328636b06d3c202ab3e0203265a371fbed36bb616579608642ea41f3124f48ea/diff"
	testrpath = "/var/lib/migration/overlay2/328636b06d3c202ab3e0203265a371fbed36bb616579608642ea41f3124f48ea/diff"
	p         = &PreMigrationInTargetMachine{
		Id:        id1,
		Cname:     "m1",
		ImageName: "mysql:5.6",
		SrcIp:     "192.168.18.129",
		Vol: []Volumes{
			struct {
				src string
				dst string
			}{src: "/opt/workdir/tmpfile/mysqlvol/data", dst: "/var/lib/mysql"},
			struct {
				src string
				dst string
			}{src: "/opt/workdir/tmpfile/custome", dst: "/etc/mysql/conf.d"},
		},
	}
)

func TestGetDir(t *testing.T) {

	lower, err := GetDir(id)
	if err != nil {
		t.Errorf("error:%v\n", err)
	}
	for _, v := range lower {
		println(v)
	}

}

func TestGetSftpClient(t *testing.T) {
	c, err := GetSftpClient(LoginUser, LoginPasswd, "192.168.18.128", 22)
	if err != nil {
		t.Errorf("sftp err:%v\n", err)
		t.Fail()
		return
	}
	if pwd, err := c.Getwd(); err == nil {
		println(pwd)
	}
	println("sftp ok")
	c.Close()
}

//func TestImage_PreCopyImage(t *testing.T) {
//	lower, err := GetDir(id)
//	if err != nil {
//		t.Error(err)
//		return
//	}
//	i := Image{
//		lowerRO: lower,
//	}
//
//	c, err := GetSftpClient(LoginUser, LoginPasswd, "192.168.18.128", 22)
//	if err != nil {
//		t.Errorf("sftp err:%v\n", err)
//		t.Fail()
//		return
//	}
//	if err = i.PreCopyImage(c); err != nil {
//		t.Error(err)
//	}
//}

func TestPathToRemote(t *testing.T) {
	localPath := testpath
	r, err := PathToRemote(localPath)
	if err != nil {
		t.Error(err)
		return
	}
	println(r)
}

func TestWalk(t *testing.T) {
	err := filepath.Walk(testpath, func(path string, info os.FileInfo, err error) error {

		//fmt.Printf("path:%v,is dir:%v,name:%v\n",path,info.IsDir(),info.Name() )
		fmt.Printf("err is %v\n", err)
		fmt.Println("path is ", path)
		return nil
	})

	if err != nil {
		t.Error(err)
	}

}

func TestAbs(t *testing.T) {

	absPath, err := filepath.Abs(testpath)
	if err != nil {
		t.Error(err)
		return
	}
	println(filepath.Base(testpath))
	println(absPath)
}

func TestRemoteCopyDir(t *testing.T) {
	if err := RemoteCopyDir(testpath, testrpath, nil); err != nil {
		t.Error(err)
		return
	}
}

func TestRemoteMkdirAll(t *testing.T) {
	c, err := GetSftpClient(LoginUser, LoginPasswd, "192.168.18.128", 22)
	if err != nil {
		t.Error(err)
		return
	}
	if err := RemoteMkdirAll(testrpath, c); err != nil {
		t.Error(err)
	}
}

//func TestRemoteCopyDirRsync(t *testing.T) {
//	re := &remoteMigration{
//		ip: "192.168.18.128",
//	}
//	l := "/opt/workdir/tmpfile/protobuf"
//	r := "/opt/workdir/tmpfile/protobuf"
//	RemoteCopyDirRsync(l, r, re)
//
//}

func TestGetVolume(t *testing.T) {
	id := "m1"
	v, err := GetVolume(id)
	if err != nil {
		t.FailNow()
	}
	fmt.Printf("v is %v\n", v)
}

func TestGetImage(t *testing.T) {
	id := "m1"
	v, err := GetImage(id)
	if err != nil {
		t.FailNow()
		return
	}
	fmt.Printf("image is %v\n", v)
}

func TestCopyUpperDir(t *testing.T) {
	p := &PreMigrationInTargetMachine{
		Id:        "123",
		Cname:     "m1",
		ImageName: "mysql:5.6",
		Vol: []Volumes{
			struct {
				src string
				dst string
			}{src: "/opt/workdir/tmpfile/mysqlvol/data", dst: "/var/lib/mysql"},
			struct {
				src string
				dst string
			}{src: "/opt/workdir/tmpfile/custome", dst: "/etc/mysql/conf.d"},
		},
	}
	//err:=p.CreateDockerContainer()
	//if err!=nil {
	//	glog.Println(err)
	//	t.FailNow()
	//	return
	//}
	err := p.CopyUpperDir()
	if err != nil {
		glog.Println(err)
		t.FailNow()
		return
	}
	return
}

func TestGetIp(t *testing.T) {
	_, err := GetIp()
	if err != nil {
		println(err)
		t.FailNow()
		return
	}
	return
}

func TestGetCName(t *testing.T) {
	c, err := GetCName("895fcefdab76")
	if err != nil {
		glog.Println(err)
		t.FailNow()
		return
	}
	glog.Printf("cname is %v\n", c)
	return
}
