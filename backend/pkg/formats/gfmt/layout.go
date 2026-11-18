package gfmt

import (
	"fmt"
	"path"
	"strings"
)

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

// FileNames hands out file names unique within a package: a name taken
// before gets a number before its extension, as in ok-2.cpp.
type FileNames map[string]bool

// Place returns a free path for name in folder and takes it.
func (f FileNames) Place(folder, name string) string {
	ext := path.Ext(name)
	file := folder + "/" + name
	for n := 2; f[file]; n++ {
		file = fmt.Sprintf("%s/%s-%d%s", folder, strings.TrimSuffix(name, ext), n, ext)
	}
	f[file] = true
	return file
}
