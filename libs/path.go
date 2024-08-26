package libs

import (
	"os"
	"path/filepath"
	"runtime"
)

func GetHome() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}

func AbsPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	currDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(currDir, path)
}
