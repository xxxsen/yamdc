package envflag

import (
	"github.com/kelseyhightower/envconfig"
)

var defaultInst = &EnvFlag{}

type EnvFlag struct {
	EnableSearchMetaCache    bool `envconfig:"ENABLE_SEARCH_META_CACHE" default:"true"`
	EnableLinkMode           bool `envconfig:"ENABLE_LINK_MODE"`
	EnableGoFaceRecognizer   bool `envconfig:"ENABLE_GO_FACE_RECOGNIZER" default:"true"`
	EnablePigoFaceRecognizer bool `envconfig:"ENABLE_PIGO_FACE_RECOGNIZER" default:"true"`
	EnableSearcherCheck      bool `envconfig:"ENABLE_SEARCHER_CHECK" default:"false"`
}

func GetFlag() *EnvFlag {
	return defaultInst
}

func Init() error {
	fg := &EnvFlag{}
	if err := envconfig.Process("yamdc", fg); err != nil {
		return err
	}
	defaultInst = fg
	return nil
}

func IsEnableSearchMetaCache() bool {
	return GetFlag().EnableSearchMetaCache
}

func IsEnableLinkMode() bool {
	return GetFlag().EnableLinkMode
}

func IsEnableGoFaceRecognizer() bool {
	return GetFlag().EnableGoFaceRecognizer
}

func IsEnablePigoFaceRecognizer() bool {
	return GetFlag().EnablePigoFaceRecognizer
}

func IsEnableSearcherCheck() bool {
	return GetFlag().EnableSearcherCheck
}
