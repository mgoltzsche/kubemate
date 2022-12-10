package controller

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

func copyManifests(srcDir, dstDir string) error {
	err := os.MkdirAll(dstDir, 0755)
	if err != nil {
		return err
	}
	dir, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range dir {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			dstFile := filepath.Join(dstDir, e.Name())
			if _, err := os.Stat(dstFile); os.IsNotExist(err) {
				err = copyFile(filepath.Join(srcDir, e.Name()), dstFile)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func copyFile(srcFile, dstFile string) error {
	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}
	defer out.Close()
	in, err := os.Open(srcFile)
	defer in.Close()
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return nil
}
