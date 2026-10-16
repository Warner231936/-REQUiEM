package modelmgr

import (
	"os"
)

func osStat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func osRename(oldPath, newPath string) error {
	return os.Rename(oldPath, newPath)
}

func osRemove(path string) (bool, error) {
	err := os.Remove(path)
	if err != nil && os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func osReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func osWriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func osMkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func osMkdirTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}

func fileExists(path string) bool {
	_, err := osStat(path)
	return err == nil
}
