package envflag

import (
	"github.com/kelseyhightower/envconfig"
)

var defaultInst = &EnvFlag{}

type EnvFlag struct {
	EnableSearchMetaCache    bool `envconfig:"enable_search_meta_cache" default:"true"`
	EnableLinkMode           bool `envconfig:"enable_link_mode"`
	EnableGoFaceRecognizer   bool `envconfig:"enable_go_face_recognizer" default:"true"`
	EnablePigoFaceRecognizer bool `envconfig:"enable_pigo_face_recognizer" default:"true"`
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
