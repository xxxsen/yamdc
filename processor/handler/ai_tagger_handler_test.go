package handler

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"yamdc/aiengine"
	"yamdc/aiengine/gemini"
	"yamdc/model"

	"github.com/stretchr/testify/assert"
)

func init() {
	raw, err := os.ReadFile("../../.vscode/keys.json")
	if err != nil {
		panic(err)
	}
	keys := make(map[string]string)
	if err := json.Unmarshal(raw, &keys); err != nil {
		panic(err)
	}
	for k, v := range keys {
		_ = os.Setenv(k, v)
	}
}

func TestAITagger(t *testing.T) {
	eng, err := gemini.New(gemini.WithKey(os.Getenv("GEMINI_KEY")), gemini.WithModel("gemini-2.0-flash"))
	assert.NoError(t, err)
	aiengine.SetAIEngine(eng)
	h := &aiTaggerHandler{}
	fctx := &model.FileContext{
		Meta: &model.MovieMeta{
			Title: "控制不住的戴绿帽冲动，得到了老公的认可！一个表情严肃，受虐狂的变态老婆，在镜头前暴露一切。她拥有美丽的乳房，乳头溢出胸罩，臀部丰满而美丽。她的阴部被她丈夫以外的人弄乱，使她潮吹。当她舔着沾满自己爱液的手指时，脸上的表情真是太色情了！这敏感到无比的小穴，只要被插入就能达到高潮吗？ ！随着另一个男人的阴茎一次又一次地出汗和射精！ [首拍] 线上应聘AV→AV体验拍摄2326",
			Plot:  "",
		},
	}
	err = h.Handle(context.Background(), fctx)
	assert.NoError(t, err)
	t.Logf("tags:%+v", fctx.Meta.Genres)
}
