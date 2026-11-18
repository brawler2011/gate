package gfmt

// The folder layout of a Gate package. This is the only place that knows it:
// the author format reader, the builder and the exporter place files here;
// drafts hold typed entries without paths.
const (
	LibraryFolder   = "lib"
	AssetFolder     = "assets"
	StatementFolder = "statements"
	TestFolder      = "tests"
)

var roleFolders = map[string]string{
	"checker": "checkers", "validator": "validators", "interactor": "interactors", "generator": "generators", "solution": "solutions",
}

// RoleFolder returns the folder that holds programs of role.
func RoleFolder(role string) string { return roleFolders[role] }

// folderRole returns the role whose folder is folder, or "".
func folderRole(folder string) string {
	for role, f := range roleFolders {
		if f == folder {
			return role
		}
	}
	return ""
}

// StatementPath returns the file of one statement section in language.
func StatementPath(language, section string) string {
	return StatementFolder + "/" + language + "/" + section + ".md"
}
