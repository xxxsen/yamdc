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
	case ".mp4", ".wav", ".avi", ".mkv", ".rmvb", ".m4a", ".ts":
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

func Move(source, destination string) error {
	err := os.Rename(source, destination)
	if err != nil && strings.Contains(err.Error(), "invalid cross-device link") {
		return moveCrossDevice(source, destination)
	}
	return err
}

func moveCrossDevice(source, destination string) error {
	dstTemp := destination + ".tempfile"
	src, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("open src:%s failed, err:%w", source, err)
	}
	defer src.Close()
	dst, err := os.Create(dstTemp)
	if err != nil {
		return fmt.Errorf("create dst:%s failed, err:%w", dstTemp, err)
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("copy src to dst failed, err:%w", err)
	}
	fi, err := os.Stat(source)
	if err != nil {
		_ = os.Remove(dstTemp)
		return fmt.Errorf("stat source failed, err:%w", err)
	}
	err = os.Chmod(dstTemp, fi.Mode())
	if err != nil {
		_ = os.Remove(dstTemp)
		return fmt.Errorf("chown dst failed, err:%w", err)
	}
	if err := os.Rename(dstTemp, destination); err != nil {
		_ = os.Remove(dstTemp)
		return fmt.Errorf("rename dst temp to dst failed, err:%w", err)
	}
	os.Remove(source)
	return nil
}
