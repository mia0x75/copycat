package path

import (
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

// check dir is exists
// if exists return true, else return false
func Exists(dir string) bool {
	dir = strings.Replace(dir, "\\", "/", -1)
	_, err := os.Stat(dir)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

// get current path
func GetCurrentPath() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Errorf("[E] %+v", err)
		return ""
	}
	return strings.Replace(dir, "\\", "/", -1)
}

// current path
var CurrentPath = GetCurrentPath()

// working dir (project dir)
var WorkingDir = getWorkingPath()

func getWorkingPath() string {
	wd, err := os.Getwd()
	if err == nil {
		workingDir := filepath.ToSlash(wd) + "/"
		return workingDir
	}
	return "/"
}

// path format, remove the last /
func GetPath(dir string) string {
	dir = strings.Replace(dir, "\\", "/", -1)
	if dir[len(dir)-1:] == "/" {
		return dir[:len(dir)-1]
	}
	return dir
}

// delete path
func Delete(dir string) bool {
	if !Exists(dir) {
		log.Warnf("[W] delete dir %s is not exists", dir)
		return false
	}
	return nil == os.RemoveAll(dir)
}
