package web

const (
	errCodeUnknown = 1

	// Common request errors.
	errCodeMethodNotAllowed      = 100001
	errCodeInvalidJSONBody       = 100002
	errCodeReadBodyFailed        = 100003
	errCodeServiceUnavailable    = 100004
	errCodeInvalidJobID          = 100005
	errCodeInvalidUploadFile     = 100006
	errCodeReadUploadFileFailed  = 100007
	errCodeUploadFileNotImage    = 100008
	errCodeInvalidCropRectangle  = 100009
	errCodeInvalidAssetTarget    = 100010
	errCodeInvalidAssetKind      = 100011
	errCodeMissingLibraryPath    = 100012
	errCodeMissingFilePath       = 100013
	errCodeMissingMediaLibraryID = 100014
	errCodeLibraryNotConfigured  = 100015
	errCodeInvalidAssetKey       = 100016
	errCodeInputRequired         = 100017

	// Job and review errors.
	errCodeScanFailed               = 110001
	errCodeListJobsFailed           = 110002
	errCodeApplyJobConflictsFailed  = 110003
	errCodeJobRunFailed             = 110004
	errCodeJobRerunFailed           = 110005
	errCodeJobLogsFailed            = 110006
	errCodeJobUpdateNumberFailed    = 110007
	errCodeJobDeleteFailed          = 110008
	errCodeReviewGetFailed          = 110101
	errCodeReviewSaveFailed         = 110102
	errCodeReviewImportFailed       = 110103
	errCodeReviewPosterCropFailed   = 110104
	errCodeReviewScrapeDataNotFound = 110105
	errCodeInvalidReviewJSON        = 110106
	errCodeReviewAssetStoreFailed   = 110107
	errCodeReviewMarshalJSONFailed  = 110108
	errCodeReviewRejectFailed       = 110109

	// Library errors.
	errCodeListLibraryFailed         = 120001
	errCodeResolveLibraryPathFailed  = 120002
	errCodeLibraryItemNotFound       = 120003
	errCodeLibraryItemReadFailed     = 120004
	errCodeLibraryUpdateFailed       = 120005
	errCodeLibraryFileNotFound       = 120006
	errCodeLibraryFileOpenFailed     = 120007
	errCodeLibraryFileDeleteDenied   = 120008
	errCodeLibraryFileDeleteFailed   = 120009
	errCodeLibraryDetailReloadFailed = 120010
	errCodeLibraryAssetReplaceFailed = 120011
	errCodeLibraryPosterCropFailed   = 120012
	errCodeLibraryItemDeleteFailed   = 120013

	// Media library errors.
	errCodeListMediaLibraryFailed         = 130001
	errCodeMediaLibraryDetailNotFound     = 130002
	errCodeMediaLibraryDetailReadFailed   = 130003
	errCodeMediaLibraryUpdateFailed       = 130004
	errCodeResolveMediaLibraryPathFailed  = 130005
	errCodeMediaLibraryFileNotFound       = 130006
	errCodeMediaLibraryFileOpenFailed     = 130007
	errCodeMediaLibraryFileDeleteFailed   = 130008
	errCodeMediaLibraryAssetReplaceFailed = 130009
	errCodeMediaLibrarySyncStatusFailed   = 130010
	errCodeMediaLibrarySyncTriggerFailed  = 130011
	errCodeMediaLibraryMoveStatusFailed   = 130012
	errCodeMediaLibraryMoveTriggerFailed  = 130013
	errCodeMediaLibraryStatusFailed       = 130014
	errCodeMediaLibrarySyncLogsFailed     = 130015

	// Debug errors.
	errCodeMovieIDCleanerUnavailable   = 140001
	errCodeMovieIDCleanerExplainFailed = 140002
	errCodeSearcherDebuggerUnavailable = 140101
	errCodeSearcherDebugSearchFailed   = 140102
	errCodePluginEditorUnavailable     = 140103
	errCodePluginEditorCompileFailed   = 140104
	errCodePluginEditorRequestFailed   = 140105
	errCodePluginEditorScrapeFailed    = 140106
	errCodePluginEditorWorkflowFailed  = 140107
	errCodePluginEditorCaseFailed      = 140108
	errCodePluginEditorImportFailed    = 140109
	errCodeHandlerDebuggerUnavailable  = 140201
	errCodeHandlerDebugRunFailed       = 140202

	// Asset errors.
	errCodeDebugAssetStoreFailed = 150001
	errCodeAssetNotFound         = 150002
)
