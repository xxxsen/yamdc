package runtime

import "errors"

var (
	ErrNoFaceRecImpl           = errors.New("no face rec impl inited")
	ErrNoEngineUsed            = errors.New("no engine used, need to check engine config")
	ErrAIEngineNotConfigured   = errors.New("ai engine is not configured")
	ErrTranslatorNotConfigured = errors.New("translator is not configured")
)
