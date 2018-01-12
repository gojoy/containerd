package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

var (
	id        string = "0a4e9597c1741c1ae755beda85461030ca87aed304292a7993f76ec4fe2a75fe"
	testpath  string = "/var/lib/docker/overlay2/328636b06d3c202ab3e0203265a371fbed36bb616579608642ea41f3124f48ea/diff"
	testrpath string = "/var/lib/migration/overlay2/328636b06d3c202ab3e0203265a371fbed36bb616579608642ea41f3124f48ea/diff"
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

func TestImage_PreCopyImage(t *testing.T) {
	lower, err := GetDir(id)
	if err != nil {
		t.Error(err)
		return
	}
	i := Image{
		lowerRO: lower,
	}


	c, err := GetSftpClient(LoginUser, LoginPasswd, "192.168.18.128", 22)
	if err != nil {
		t.Errorf("sftp err:%v\n", err)
		t.Fail()
		return
	}
	if err = i.PreCopyImage(c); err != nil {
		t.Error(err)
	}
}

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
	if err!=nil {
		t.Error(err)
		return
	}
	if err:=RemoteMkdirAll(testrpath,c);err!=nil {
		t.Error(err)
	}
}
