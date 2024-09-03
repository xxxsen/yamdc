package downloadmgr

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"yamdc/client"
)

type DownloadManager struct {
	cli client.IHTTPClient
}

func NewManager(cli client.IHTTPClient) *DownloadManager {
	return &DownloadManager{cli: cli}
}

func (m *DownloadManager) ensureDir(dst string) error {
	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir failed, path:%s, err:%w", dir, err)
	}
	return nil
}

func (m *DownloadManager) createHTTPStream(src string) (io.ReadCloser, error) {
	req, err := http.NewRequest(http.MethodGet, src, nil)
	if err != nil {
		return nil, err
	}
	rsp, err := m.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request failed, err:%w", err)
	}
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code:%d not ok", rsp.StatusCode)
	}
	rc, err := client.BuildReaderFromHTTPResponse(rsp)
	if err != nil {
		rsp.Body.Close()
		return nil, fmt.Errorf("build reader failed, err:%w", err)
	}
	return rc, nil
}

func (m *DownloadManager) writeToFile(rc io.Reader, dst string) error {
	tmp := dst + ".temp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("open temp file for read failed, err:%w", err)
	}

	if _, err := io.Copy(f, rc); err != nil {
		_ = f.Close()
		return fmt.Errorf("transfer data failed, err:%w", err)
	}
	_ = f.Close()
	if err := os.Rename(tmp, dst); err != nil {
		return fmt.Errorf("unable to move file:%w", err)
	}
	return nil
}

func (m *DownloadManager) Download(src string, dst string) error {
	if err := m.ensureDir(dst); err != nil {
		return err
	}
	rc, err := m.createHTTPStream(src)
	if err != nil {
		return err
	}
	defer rc.Close()
	if err := m.writeToFile(rc, dst); err != nil {
		return err
	}
	return nil
}
