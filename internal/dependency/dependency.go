package dependency

import (
	"context"
	"fmt"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/downloadmgr"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type Dependency struct {
	URL     string
	Target  string
	Refresh bool
}

func Resolve(cli client.IHTTPClient, deps []*Dependency) error {
	m := downloadmgr.NewManager(cli)
	for _, dep := range deps {
		if err := handleFileDownload(context.Background(), m, dep); err != nil {
			return fmt.Errorf("download link:%s to target:%s failed, err:%w", dep.URL, dep.Target, err)
		}
	}
	return nil
}

func handleFileDownload(ctx context.Context, m *downloadmgr.DownloadManager, dep *Dependency) error {
	updated, err := m.Download(ctx, dep.URL, dep.Target, dep.Refresh)
	if err != nil {
		return err
	}
	logutil.GetLogger(ctx).Debug("dependency sync succ", zap.String("link", dep.URL), zap.String("target", dep.Target), zap.Bool("updated", updated))
	return nil
}
