package lazycopydir

import (
	"path/filepath"
	"os"
	"os/exec"
	"log"
)

//merge upper and lazy,we must umount mergedir before do it
func MergeLazyDir(upper, lazy, mergedir string) error {

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
	log.Printf("err is %v,out is %v\n", err, string(buf))
	if err != nil {
		log.Printf("err is %v,out is %v\n", err, string(buf))
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
	return nil
}
