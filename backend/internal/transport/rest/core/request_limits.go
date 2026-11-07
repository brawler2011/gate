package core

const (
	DefaultRequestBodyLimit = 1 << 20 // 1 MiB for JSON and other small requests.
	// Multipart framing and metadata have a separate, bounded allowance.
	multipartOverheadLimit = 1 << 20
)

// RequestBodyLimit selects limits by operation rather than client Content-Type.
// Uploads retain their larger file limits; raw uploads remain streaming readers.
func RequestBodyLimit(operationID string) int64 {
	switch operationID {
	case "ImportProblem":
		return MaxPackageZipSize + multipartOverheadLimit
	case "UploadAvatar", "CreatePost", "PatchPostById":
		return MaxMediaFileSize + multipartOverheadLimit
	case "CreateProblemMediaFile", "UpdateProblemMediaFile":
		return MaxMediaFileSize
	case "CreateProblemTestFile", "UpdateProblemTestFile":
		return MaxTestFileSize
	case "CreateProblemChecker", "UpdateProblemChecker",
		"CreateProblemGenerator", "UpdateProblemGenerator",
		"CreateProblemInteractor", "UpdateProblemInteractor",
		"CreateProblemValidator", "UpdateProblemValidator",
		"CreateProblemLib", "UpdateProblemLib",
		"CreateProblemWorkshopSubmission", "UpdateProblemWorkshopSubmission":
		return MaxSourceFileSize
	default:
		return DefaultRequestBodyLimit
	}
}
