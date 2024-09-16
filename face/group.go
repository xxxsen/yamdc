package face

import (
	"context"
	"image"

	"github.com/xxxsen/common/logutil"
	"go.uber.org/zap"
)

type group struct {
	impls []IFaceRec
}

func NewGroup(impls []IFaceRec) IFaceRec {
	return &group{impls: impls}
}

func (g *group) Name() string {
	return "group"
}

func (g *group) SearchFaces(ctx context.Context, data []byte) ([]image.Rectangle, error) {
	var retErr error
	for _, impl := range g.impls {
		recs, err := impl.SearchFaces(ctx, data)
		if err == nil {
			logutil.GetLogger(ctx).Debug("search face succ", zap.String("face_rec_impl", impl.Name()))
			return recs, nil
		}
		retErr = err
	}
	return nil, retErr
}
