package runtime

import (
	"context"

	"github.com/xxxsen/common/logutil"
	"github.com/xxxsen/yamdc/internal/face"
	"github.com/xxxsen/yamdc/internal/face/pigo"
	"go.uber.org/zap"
)

func BuildFaceRecognizer(ctx context.Context, enablePigo bool, models string) (face.IFaceRec, error) {
	impls := make([]face.IFaceRec, 0, 2)
	faceRecCreator := make([]func() (face.IFaceRec, error), 0, 2)
	if enablePigo {
		faceRecCreator = append(faceRecCreator, func() (face.IFaceRec, error) {
			return pigo.NewPigo(models)
		})
	}
	for index, creator := range faceRecCreator {
		impl, err := creator()
		if err != nil {
			logutil.GetLogger(ctx).Error("create face rec impl failed", zap.Int("index", index), zap.Error(err))
			continue
		}
		logutil.GetLogger(ctx).Info("use face recognizer", zap.String("name", impl.Name()))
		impls = append(impls, impl)
	}
	if len(impls) == 0 {
		return nil, ErrNoFaceRecImpl
	}
	return face.NewGroup(impls), nil
}
