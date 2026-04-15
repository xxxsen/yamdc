package capture

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/uuid"
)

func moveFile(srcFile, dstFile string) error {
	err := os.Rename(srcFile, dstFile)
	if err != nil && strings.Contains(err.Error(), "invalid cross-device link") {
		return moveCrossDevice(srcFile, dstFile)
	}
	if err != nil {
		return fmt.Errorf("rename %s to %s failed: %w", srcFile, dstFile, err)
	}
	return nil
}

func copyFile(srcFile, dstFile string) error {
	fi, err := os.Stat(srcFile)
	if err != nil {
		return fmt.Errorf("stat source failed, err:%w", err)
	}
	src, err := os.Open(srcFile)
	if err != nil {
		return fmt.Errorf("open src:%s failed, err:%w", srcFile, err)
	}
	defer func() {
		_ = src.Close()
	}()
	dst, err := os.OpenFile(dstFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create dst:%s failed, err:%w", dstFile, err)
	}
	defer func() {
		_ = dst.Close()
	}()
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
	defer func() {
		_ = os.Remove(dstFileTemp)
	}()
	if err := copyFile(srcFile, dstFileTemp); err != nil {
		return fmt.Errorf("copy file failed, err:%w", err)
	}
	if err := os.Rename(dstFileTemp, dstFile); err != nil {
		return fmt.Errorf("rename dst temp to dst failed, err:%w", err)
	}
	_ = os.Remove(srcFile)
	return nil
}
