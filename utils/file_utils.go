package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func IsVideoFile(f string) bool {
	ext := strings.ToLower(filepath.Ext(f))
	switch ext {
	case ".mp4", ".wmv", ".avi", ".mkv", ".rmvb", ".m4a", ".ts":
		return true
	default:
		return false
	}
}

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

func moveCrossDevice(srcFile, dstFile string) error {
	dstFileTemp := dstFile + ".tempfile"
	src, err := os.Open(srcFile)
	if err != nil {
		return fmt.Errorf("open src:%s failed, err:%w", srcFile, err)
	}
	defer src.Close()
	dst, err := os.Create(dstFileTemp)
	if err != nil {
		return fmt.Errorf("create dst:%s failed, err:%w", dstFileTemp, err)
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("copy src to dst failed, err:%w", err)
	}
	fi, err := os.Stat(srcFile)
	if err != nil {
		_ = os.Remove(dstFileTemp)
		return fmt.Errorf("stat source failed, err:%w", err)
	}
	err = os.Chmod(dstFileTemp, fi.Mode())
	if err != nil {
		_ = os.Remove(dstFileTemp)
		return fmt.Errorf("chown dst failed, err:%w", err)
	}
	if err := os.Rename(dstFileTemp, dstFile); err != nil {
		_ = os.Remove(dstFileTemp)
		return fmt.Errorf("rename dst temp to dst failed, err:%w", err)
	}
	os.Remove(srcFile)
	return nil
}
