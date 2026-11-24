package gfmt

// Draft entries have no paths; readers, builders and exporters share this package layout.
const (
	LibraryFolder   = "lib"
	GraderFolder    = "graders"
	// Attachments are given to participants.
	AttachmentFolder = "attachments"
	AssetFolder     = "assets"
	StatementFolder = "statements"
	TestFolder      = "tests"
)

var roleFolders = map[string]string{
	"checker": "checkers", "validator": "validators", "interactor": "interactors", "generator": "generators", "solution": "solutions",
}

func RoleFolder(role string) string { return roleFolders[role] }

func folderRole(folder string) string {
	for role, f := range roleFolders {
		if f == folder {
			return role
		}
	}
	return ""
}

func StatementPath(language, section string) string {
	return StatementFolder + "/" + language + "/" + section + ".tex"
}
