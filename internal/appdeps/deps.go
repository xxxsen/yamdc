package appdeps

import (
	"github.com/xxxsen/yamdc/internal/aiengine"
	"github.com/xxxsen/yamdc/internal/client"
	"github.com/xxxsen/yamdc/internal/face"
	"github.com/xxxsen/yamdc/internal/store"
	"github.com/xxxsen/yamdc/internal/translator"
)

type Runtime struct {
	HTTPClient client.IHTTPClient
	Storage    store.IStorage
	Translator translator.ITranslator
	AIEngine   aiengine.IAIEngine
	FaceRec    face.IFaceRec
}
