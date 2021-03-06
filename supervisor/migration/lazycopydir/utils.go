package lazycopydir

import (
	"golang.org/x/sys/unix"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	S_IFMT    = 00170000
	S_IFCHR   = 0020000
	WHITE_DEV = 0

	ovl_opaque_xattr = "trusted.overlay.opaque"
)

//stat.mode 32位，目前只用了低16位，其中前4位代表文件类别，后12位代表权限
//stat.rdev =0 whiteout file
func isWhiteOut(name string) bool {
	var (
		stat unix.Stat_t
		err  error
	)

	if err = unix.Stat(name, &stat); err != nil {
		log.Println(err)
		return false
	}

	return (stat.Mode&S_IFCHR) == S_IFCHR && stat.Rdev == WHITE_DEV
}

func isOpaque(name string) bool {

	var (
		err error
		buf = make([]byte, 1)
		n   int
	)

	if n, err = unix.Getxattr(name, ovl_opaque_xattr, buf); err != nil {
		//log.Println(err)
		return false
	}

	return n == 1 && buf[0] == 'y'
}

//判断upperdir中create事件是否为在lowerdir中已经存在的目录，若是，则证明其目录不需要传输
func isDirInPath(path, crawdir string) bool {
	crawpath := filepath.Join(crawdir, path)
	info, err := os.Stat(crawpath)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return true
	}
	return false
}

func HandleCreateEvents(fullpath, path, monitordir, crawlerdir string, list *JobList) error {
	var (
		err error
	)
	//log.Printf("fullpath is %v,path is %v\n",fullpath,path)

	if isWhiteOut(fullpath) {
		//create file is whiteout
		if isDirInPath(path, crawlerdir) {
			//path is dir in lowerdir, rm -fr path
			if err = list.RemoveAllDir(path + "/"); err != nil {
				log.Println(err)
			}

		} else {
			//rm file
			if err = list.Remove(path); err != nil {
				log.Println(err)
			}

		}

	} else {
		//create file is not a whiteout
		if isDirInPath(path, monitordir) {
			//is a dir in upperdir
			if isOpaque(fullpath) {
				//is a opaque dir in upperdir,we removealldir
				if err = list.RemoveAllDir(path + "/"); err != nil {
					log.Println(err)
				}

			} else {
				//	remove all file in this upper dir
				infos, err := ioutil.ReadDir(fullpath)
				if err != nil {
					log.Println(err)
					return err
				}

				for _, v := range infos {
					log.Printf("in utils v is %v--------------------\n", v.Name())

					base, err := filepath.Rel(monitordir, fullpath)
					if err != nil {
						log.Println(err)
						continue
					}
					npath := filepath.Join(base, v.Name())
					fpath := filepath.Join(fullpath, v.Name())
					if err = HandleCreateEvents(fpath, npath, monitordir, crawlerdir, list); err != nil {
						log.Println(err)
					}
				}
			}

		} else {
			//is not a dir in upper,we remove it
			if err = list.Remove(path); err != nil {
				log.Println(err)
			}

		}
	}
	return nil
}

//merge upper and lazy,we must umount mergedir before do it
func MergeDir(upper, lazy, mergedir string) error {

	var (
		err error
	)

	if err = filepath.Walk(upper, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Println(err)
			return err
		}
		if !info.IsDir() {
			if isWhiteOut(path) {
				if err = os.Remove(path); err != nil {
					log.Println(err)
				}
			}
		}
		return nil
	}); err != nil {
		log.Println(err)
		return err
	}

	if lazy[len(lazy)-1] != '/' {
		lazy = lazy + "/"
	}

	if upper[len(upper)-1] != '/' {
		upper = upper + "/."
	} else {
		upper = upper + "."
	}

	//args := []string{"-lR", "--remove-destination"}
	args := []string{"-av"}
	args = append(args, upper, lazy)
	cmd := exec.Command("rsync", args...)
	log.Printf("cmd is %v\n", cmd.Args)

	buf, err := cmd.CombinedOutput()
	log.Printf("err is %v,out is :%v\n", err, string(buf))
	if err != nil {
		log.Printf("err is %v,out is :%v\n", err, string(buf))
		return err
	}
	//if err = cmd.Run(); err != nil {
	//	log.Println(err)
	//	return err
	//}
	if err = os.RemoveAll(mergedir); err != nil {
		log.Println(err)
		return err
	}
	if err = os.Rename(lazy, mergedir); err != nil {
		log.Println(err)
		return err
	}
	log.Println("finish merge!")
	return nil
}

func UmountDir(dir string) error {
	cmd := exec.Command("umount", dir)
	log.Printf("cmd is %v\n", cmd.Args)
	if err := cmd.Run(); err != nil {
		log.Println(err)
		return err
	}
	return nil
}
