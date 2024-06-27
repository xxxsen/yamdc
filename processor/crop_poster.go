package processor

import (
	"av-capture/model"
	"context"
)

type cropPoster struct {
}

func (p *cropPoster) Name() string {
	return "crop_poster"
}

func (p *cropPoster) Process(ctx context.Context, meta *model.AvMeta) error {
	//TODO: 实现封面裁剪
	panic("not implemented") // TODO: Implement
}

func (p *cropPoster) IsOptional() bool {
	return false
}
