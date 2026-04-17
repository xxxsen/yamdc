package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	bootrt "github.com/xxxsen/yamdc/internal/bootstrap/runtime"
)

// 运行时 (runtime) 层 action 集合:
//   - AI engine / Translator / Face Recognizer
//
// 共性: 这三类都是"可选组件", 未配置或未启用时不应导致整个启动失败;
// translator / face recognizer 的初始化失败仅记录 log 并继续。

func buildAIEngineAction(ctx context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	engine, err := bootrt.BuildAIEngine(ctx, sc.Infra.HTTPClient, c.AIEngine.Name, c.AIEngine.Args)
	if err != nil && !errors.Is(err, bootrt.ErrAIEngineNotConfigured) {
		return fmt.Errorf("build ai engine: %w", err)
	}
	sc.Runtime.AIEngine = engine
	return nil
}

func buildTranslatorAction(ctx context.Context, sc *StartContext) error {
	if !sc.Infra.Config.TranslateConfig.Enable {
		return nil
	}
	tr, err := bootrt.BuildTranslator(ctx, toTranslatorConfig(sc.Infra.Config), sc.Runtime.AIEngine)
	if err != nil && !errors.Is(err, bootrt.ErrTranslatorNotConfigured) {
		logOptionalSetupFailure(ctx, sc, "setup translator failed", err)
		return nil
	}
	sc.Runtime.Translator = tr
	return nil
}

func buildFaceRecognizerAction(ctx context.Context, sc *StartContext) error {
	c := sc.Infra.Config
	faceRec, err := bootrt.BuildFaceRecognizer(
		ctx, c.SwitchConfig.EnablePigoFaceRecognizer,
		filepath.Join(c.DataDir, "models"),
	)
	if err != nil {
		logOptionalSetupFailure(ctx, sc, "init face recognizer failed", err)
		return nil
	}
	sc.Runtime.FaceRec = faceRec
	return nil
}
