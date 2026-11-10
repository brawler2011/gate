package core

const (
	// MaxMediaFileSize bounds avatar and blog images.
	MaxMediaFileSize        = 50 << 20
	DefaultRequestBodyLimit = 1 << 20 // 1 MiB for JSON and other small requests.
	// Multipart framing and metadata have a separate, bounded allowance.
	multipartOverheadLimit = 1 << 20
)

// RequestBodyLimit selects limits by operation rather than client Content-Type.
// Uploads retain their larger file limits; raw uploads remain streaming readers.
func RequestBodyLimit(operationID string) int64 {
	switch operationID {
	case "UploadAvatar", "CreatePost", "PatchPostById":
		return MaxMediaFileSize + multipartOverheadLimit
	default:
		return DefaultRequestBodyLimit
	}
}
