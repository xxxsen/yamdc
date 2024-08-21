package searcher

import (
	"context"
	"yamdc/model"
	"yamdc/number"
)

type categorySearcher struct {
	defSearcher  []ISearcher
	catSearchers map[number.Category][]ISearcher
}

func NewCategorySearcher(def []ISearcher, cats map[number.Category][]ISearcher) ISearcher {
	return &categorySearcher{defSearcher: def, catSearchers: cats}
}

func (s *categorySearcher) Name() string {
	return "category"
}

func (s *categorySearcher) Search(ctx context.Context, number *number.Number) (*model.AvMeta, bool, error) {
	cat := number.GetCategory()
	//没分类, 那么使用主链进行查询
	//存在分类, 但是分类对应的链没有配置, 则使用主链进行查询
	//如果已经存在分类链, 则不再进行降级
	chain, ok := s.catSearchers[cat]
	if !ok {
		chain = s.defSearcher
	}
	return performGroupSearch(ctx, number, chain)
}
