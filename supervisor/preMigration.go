package supervisor

import (
	"github.com/containerd/containerd/supervisor/migration"
	"github.com/sirupsen/logrus"
)

const preVolume = "/var/lib/migration/mvolume"

//目标容器数据卷的路径： /var/lib/migration/mvolume/id/{lazy,upper,work,merge}
type PreMigrationTask struct {
	baseTask
	Id        string
	UpperId   string
	ImageName string
	Vol       []Volumes
	Cname     string
	SrcIp     string
}

type Volumes struct {
	Src string
	Dst string
}

//在目的主机执行的准备操作
// 1 nfs挂载vol卷 2 docker create 目标容器 3 复制upperdir到容器的upperdir
func (s *Supervisor) PreMigration(t *PreMigrationTask) error {

	var (
		err error
	)
	vols := make([]migration.Volumes, 0)
	for _, v := range t.Vol {
		v := migration.NewVolumes(v.Src, v.Dst)
		vols = append(vols, v)
	}

	pre := &migration.PreMigrationInTargetMachine{
		Id:        t.Id,
		UpperId:   t.UpperId,
		Cname:     t.Cname,
		ImageName: t.ImageName,
		Vol:       vols,
		SrcIp:     t.SrcIp,
	}

	logrus.Println("start preMigration")
	if err = pre.StartPre(); err != nil {
		logrus.Printf("start pre in supervisor error:%v\n", err)
		return err
	}

	return nil
}

//func (p *PreMigrationTask) createDockerContainers() error {
//	var (
//		err error
//	)
//
//	args := append([]string{"create", "-P", "--security-opt", "seccomp:unconfined", "-e", "MYSQL_ROOT_PASSWORD=123456", "--name"},
//		p.Cname+"copy")
//	//args=append(args,"-v")
//	for _, v := range p.Vol {
//		args = append(args, "-v", fmt.Sprintf("%s:%s", v.Src, v.Dst))
//	}
//	args = append(args, p.ImageName)
//	cmd := exec.Command("docker", args...)
//	if err = cmd.Run(); err != nil {
//		logrus.Println(err)
//		return err
//	}
//	return nil
//}
