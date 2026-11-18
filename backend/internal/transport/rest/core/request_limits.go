package core

const (
	MaxMediaFileSize        = 50 << 20
	DefaultRequestBodyLimit = 1 << 20
	// Multipart framing and metadata have a separate, bounded allowance.
	multipartOverheadLimit = 1 << 20
)

// Choose body limits by operation so client Content-Type cannot change the allowance.
func RequestBodyLimit(operationID string) int64 {
	switch operationID {
	case "UploadAvatar", "CreatePost", "PatchPostById":
		return MaxMediaFileSize + multipartOverheadLimit
	default:
		return DefaultRequestBodyLimit
	}
}
