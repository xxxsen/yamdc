package handler

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"yamdc/internal/aiengine"
	"yamdc/internal/aiengine/gemini"
	"yamdc/internal/model"

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
	titles := []string{
		"控制不住的戴绿帽冲动，得到了老公的认可！一个表情严肃，受虐狂的变态老婆，在镜头前暴露一切。她拥有美丽的乳房，乳头溢出胸罩，臀部丰满而美丽。她的阴部被她丈夫以外的人弄乱，使她潮吹。当她舔着沾满自己爱液的手指时，脸上的表情真是太色情了！这敏感到无比的小穴，只要被插入就能达到高潮吗？ ！随着另一个男人的阴茎一次又一次地出汗和射精！ [首拍] 线上应聘AV→AV体验拍摄2326",
		"MTALL-148 うるちゅるリップで締めつけるジュル音高めの極上スローフェラと淫語性交 雫月心桜",
		"MKMP-626 顔面レベル100 小那海あやの4コス＆4シチュで男性を最高の快感に導く ささやき淫語射精サポート",
		"JUR-036 超大型新人 専属第2章 中出し解禁！！ 夫と子作りSEXをした後はいつも義父に中出しされ続けています…。 新妻ゆうか",
		"上司の目を盗んで同僚とこっそり…社内不倫の背徳感に溺れて",
		"风骚女秘书深夜主动诱惑上司，在办公室掀起欲望风暴",
	}
	for idx, title := range titles {
		h := &aiTaggerHandler{}
		fctx := &model.FileContext{
			Meta: &model.MovieMeta{
				Title: title,
				Plot:  "",
			},
		}
		err = h.Handle(context.Background(), fctx)
		assert.NoError(t, err)
		t.Logf("index:%d, tags:%+v", idx, fctx.Meta.Genres)
	}
}
