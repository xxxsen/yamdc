package jobdef

type Status string

const (
	StatusInit       Status = "init"
	StatusProcessing Status = "processing"
	StatusReviewing  Status = "reviewing"
	StatusDone       Status = "done"
	StatusFailed     Status = "failed"
)

type Job struct {
	ID                    int64  `json:"id"`
	JobUID                string `json:"job_uid"`
	FileName              string `json:"file_name"`
	FileExt               string `json:"file_ext"`
	RelPath               string `json:"rel_path"`
	AbsPath               string `json:"abs_path"`
	Number                string `json:"number"`
	RawNumber             string `json:"raw_number"`
	CleanedNumber         string `json:"cleaned_number"`
	NumberSource          string `json:"number_source"`
	NumberCleanStatus     string `json:"number_clean_status"`
	NumberCleanConfidence string `json:"number_clean_confidence"`
	NumberCleanWarnings   string `json:"number_clean_warnings"`
	FileSize              int64  `json:"file_size"`
	Status                Status `json:"status"`
	ErrorMsg              string `json:"error_msg"`
	CreatedAt             int64  `json:"created_at"`
	UpdatedAt             int64  `json:"updated_at"`
}
