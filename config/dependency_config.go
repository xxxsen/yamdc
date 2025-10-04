package config

var sysDependencies = []Dependency{
	{Link: "https://github.com/Kagami/go-face-testdata/raw/master/models/shape_predictor_5_face_landmarks.dat", RelPath: "models/shape_predictor_5_face_landmarks.dat"},
	{Link: "https://github.com/Kagami/go-face-testdata/raw/master/models/dlib_face_recognition_resnet_model_v1.dat", RelPath: "models/dlib_face_recognition_resnet_model_v1.dat"},
	{Link: "https://github.com/Kagami/go-face-testdata/raw/master/models/mmod_human_face_detector.dat", RelPath: "models/mmod_human_face_detector.dat"},
	{Link: "https://github.com/esimov/pigo/raw/master/cascade/facefinder", RelPath: "models/facefinder"},
	{Link: "https://raw.githubusercontent.com/xxxsen/yamdc-script/refs/heads/master/scripts/number_rewriter.toml", RelPath: "scripts/number_rewriter.toml", Refresh: true},
	{Link: "https://raw.githubusercontent.com/xxxsen/yamdc-script/refs/heads/master/scripts/number_categorier.toml", RelPath: "scripts/number_categorier.toml", Refresh: true},
	{Link: "https://raw.githubusercontent.com/xxxsen/yamdc-script/refs/heads/master/scripts/number_uncensor_tester.toml", RelPath: "scripts/number_uncensor_tester.toml", Refresh: true},
}

var sysRuleConfig = RuleConfig{
	NumberRewriterConfig:       "scripts/number_rewriter.toml",
	NumberCategorierConfig:     "scripts/number_categorier.toml",
	NumberUncensorTesterConfig: "scripts/number_uncensor_tester.toml",
}
