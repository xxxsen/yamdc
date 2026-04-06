package searcher

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/xxxsen/yamdc/internal/model"
	"github.com/xxxsen/yamdc/internal/number"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type RuntimeCategorySearcher struct {
	mu           sync.RWMutex
	defSearcher  []ISearcher
	catSearchers map[string][]ISearcher
}

func NewCategorySearcher(def []ISearcher, cats map[string][]ISearcher) *RuntimeCategorySearcher {
	s := &RuntimeCategorySearcher{}
	s.Swap(def, cats)
	return s
}

func (s *RuntimeCategorySearcher) Swap(def []ISearcher, cats map[string][]ISearcher) {
	nextCats := make(map[string][]ISearcher, len(cats))
	for key, items := range cats {
		nextCats[strings.ToUpper(strings.TrimSpace(key))] = append([]ISearcher(nil), items...)
	}
	s.mu.Lock()
	s.defSearcher = append([]ISearcher(nil), def...)
	s.catSearchers = nextCats
	s.mu.Unlock()
}

func (s *RuntimeCategorySearcher) Name() string {
	return "category"
}

func (s *RuntimeCategorySearcher) Check(ctx context.Context) error {
	return fmt.Errorf("unable to perform check on category searcher")
}

func (s *RuntimeCategorySearcher) Search(ctx context.Context, n *number.Number) (*model.MovieMeta, bool, error) {
	cat := n.GetExternalFieldCategory()
	//没分类, 那么使用主链进行查询
	//存在分类, 但是分类对应的链没有配置, 则使用主链进行查询
	//如果已经存在分类链, 则不再进行降级
	logger := logutil.GetLogger(ctx).With(zap.String("cat", string(cat)))
	s.mu.RLock()
	chain := append([]ISearcher(nil), s.defSearcher...)
	catChains := s.catSearchers
	s.mu.RUnlock()
	if len(cat) > 0 {
		if c, ok := catChains[strings.ToUpper(strings.TrimSpace(string(cat)))]; ok {
			chain = append([]ISearcher(nil), c...)
			logger.Debug("use cat chain for search")
		} else {
			logger.Error("no cat chain found, use default plugin chain for search")
		}
	}

	return performGroupSearch(ctx, n, chain)
}
