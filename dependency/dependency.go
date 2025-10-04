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
	URL     string
	Target  string
	Refresh bool
}

func Resolve(cli client.IHTTPClient, deps []*Dependency) error {
	m := downloadmgr.NewManager(cli)
	for _, dep := range deps {
		if err := checkAndDownload(context.Background(), m, dep); err != nil {
			return fmt.Errorf("download link:%s to target:%s failed, err:%w", dep.URL, dep.Target, err)
		}
	}
	return nil
}

func checkAndDownload(ctx context.Context, m *downloadmgr.DownloadManager, dep *Dependency) error {
	target := dep.Target
	if dep.Refresh {
		updated, err := m.Sync(ctx, dep.URL, target)
		if err != nil {
			return err
		}
		if updated {
			logutil.GetLogger(ctx).Info("dependency updated", zap.String("link", dep.URL), zap.String("target", target))
			if err := writeTimestamp(target); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := os.Stat(target + defaultSuffix); err == nil {
		return nil
	}
	logutil.GetLogger(ctx).Debug("start download link", zap.String("link", dep.URL))
	if err := m.Download(dep.URL, target); err != nil {
		return err
	}
	logutil.GetLogger(ctx).Debug("download link succ", zap.String("link", dep.URL))
	if err := writeTimestamp(target); err != nil {
		return err
	}
	return nil
}

func writeTimestamp(target string) error {
	if err := os.WriteFile(target+defaultSuffix, []byte(fmt.Sprintf("%d", time.Now().Unix())), 0644); err != nil {
		return fmt.Errorf("write ts file failed, err:%w", err)
	}
	return nil
}
