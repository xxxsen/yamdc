package dependency

import (
	"context"
	"fmt"
	"os"
	"time"
	"yamdc/client"
	"yamdc/downloadmgr"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

const (
	defaultSuffix = ".ts"
)

type Dependency struct {
	URL    string
	Target string
}

func Resolve(cli client.IHTTPClient, deps []*Dependency) error {
	m := downloadmgr.NewManager(cli)
	for _, dep := range deps {
		if err := checkAndDownload(m, dep.URL, dep.Target); err != nil {
			return fmt.Errorf("download link:%s to target:%s failed, err:%w", dep.URL, dep.Target, err)
		}
	}
	return nil
}

func checkAndDownload(m *downloadmgr.DownloadManager, link string, target string) error {
	if _, err := os.Stat(target + defaultSuffix); err == nil {
		return nil
	}
	logutil.GetLogger(context.Background()).Debug("start download link", zap.String("link", link))
	if err := m.Download(link, target); err != nil {
		return err
	}
	logutil.GetLogger(context.Background()).Debug("download link succ", zap.String("link", link))
	if err := os.WriteFile(target+defaultSuffix, []byte(fmt.Sprintf("%d", time.Now().Unix())), 0644); err != nil {
		return fmt.Errorf("write ts file failed, err:%w", err)
	}
	return nil
}
