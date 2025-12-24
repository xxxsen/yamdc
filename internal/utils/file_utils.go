package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

func GetExtName(f string, def string) string {
	if v := filepath.Ext(f); v != "" {
		return v
	}
	return def
}

func Move(srcFile, dstFile string) error {
	err := os.Rename(srcFile, dstFile)
	if err != nil && strings.Contains(err.Error(), "invalid cross-device link") {
		return moveCrossDevice(srcFile, dstFile)
	}
	return err
}

func Copy(srcFile, dstFile string) error {
	fi, err := os.Stat(srcFile)
	if err != nil {
		return fmt.Errorf("stat source failed, err:%w", err)
	}
	src, err := os.Open(srcFile)
	if err != nil {
		return fmt.Errorf("open src:%s failed, err:%w", srcFile, err)
	}
	defer src.Close()
	dst, err := os.OpenFile(dstFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("create dst:%s failed, err:%w", dstFile, err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy stream failed, err:%w", err)
	}
	err = os.Chmod(dstFile, fi.Mode())
	if err != nil {
		return fmt.Errorf("chown dst failed, err:%w", err)
	}
	return nil
}

func moveCrossDevice(srcFile, dstFile string) error {
	dstFileTemp := dstFile + ".tempfile." + uuid.NewString()
	defer os.Remove(dstFileTemp)
	if err := Copy(srcFile, dstFileTemp); err != nil {
		return fmt.Errorf("copy file failed, err:%w", err)
	}
	if err := os.Rename(dstFileTemp, dstFile); err != nil {
		return fmt.Errorf("rename dst temp to dst failed, err:%w", err)
	}
	os.Remove(srcFile)
	return nil
}
