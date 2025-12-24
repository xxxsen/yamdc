package downloadmgr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"github.com/xxxsen/yamdc/internal/client"
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

type fileMeta struct {
	ETag         string `json:"etag"`
	LastModified string `json:"last_modified"`
}

func metaFilePath(dst string) string {
	return dst + ".meta"
}

func readFileMeta(path string) (*fileMeta, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var meta fileMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func (m *DownloadManager) writeFileMeta(path string, meta *fileMeta) error {
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return m.writeToFile(bytes.NewReader(raw), path)
}

func (m *DownloadManager) attachETag(req *http.Request, metaPath string) error {
	meta, err := readFileMeta(metaPath)
	if err != nil {
		return fmt.Errorf("read meta failed, err:%w", err)
	}
	if meta == nil {
		return nil
	}
	if meta.ETag != "" {
		req.Header.Set("If-None-Match", meta.ETag)
	}
	if meta.LastModified != "" {
		req.Header.Set("If-Modified-Since", meta.LastModified)
	}
	return nil
}

func (m *DownloadManager) isFileExist(dst string) bool {
	_, err := os.Stat(dst)
	return err == nil
}

func (m *DownloadManager) Download(ctx context.Context, src string, dst string, sync bool) (bool, error) {
	if m.isFileExist(dst) && !sync {
		return false, nil
	}
	if err := m.ensureDir(dst); err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return false, err
	}
	if err := m.attachETag(req, metaFilePath(dst)); err != nil {
		return false, fmt.Errorf("attach etag meta data failed, err:%w", err)
	}
	rsp, err := m.cli.Do(req)
	if err != nil {
		return false, fmt.Errorf("do request failed, err:%w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode == http.StatusNotModified {
		return false, nil
	}
	if rsp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("unexpected status code:%d", rsp.StatusCode)
	}
	rc, err := client.BuildReaderFromHTTPResponse(rsp)
	if err != nil {
		return false, fmt.Errorf("build reader failed, err:%w", err)
	}
	defer rc.Close()
	if err := m.writeToFile(rc, dst); err != nil {
		return false, err
	}
	newMeta := &fileMeta{
		ETag:         rsp.Header.Get("ETag"),
		LastModified: rsp.Header.Get("Last-Modified"),
	}
	if err := m.writeFileMeta(metaFilePath(dst), newMeta); err != nil {
		return false, err
	}
	return true, nil
}
