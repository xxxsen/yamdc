package utils

import "yamdc/model"

func EnableDataTranslate(meta *model.MovieMeta) {
	meta.ExtInfo.TranslateInfo.Plot.Enable = true
	meta.ExtInfo.TranslateInfo.Title.Enable = true
}
