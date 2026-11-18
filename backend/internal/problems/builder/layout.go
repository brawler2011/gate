package builder

import (
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
)

// placedProgram is a draft program at its package path. Key identifies it
// in the build: the role of a single checker or interactor, else its ID.
type placedProgram struct {
	key     string
	role    string
	path    string
	program model.DraftProgram
}

// packageLayout is a draft placed in the Gate package layout.
type packageLayout struct {
	texts    map[string]string
	programs []placedProgram
	assets   map[string]uuid.UUID
}

// placeDraft decides where each draft entry goes in the built package. Names
// are labels, so entries whose file names collide are numbered.
// A program's file gets the first extension of its language configuration.
func placeDraft(d model.Draft, languages *sandbox.Config) packageLayout {
	l := packageLayout{texts: map[string]string{}, assets: map[string]uuid.UUID{}}
	names := gfmt.FileNames{}
	for _, f := range d.Library {
		l.texts[names.Place(gfmt.LibraryFolder, f.Name)] = f.Text()
	}
	place := func(key, role, name string, p model.DraftProgram) {
		path := names.Place(gfmt.RoleFolder(role), name+extension(languages, p.Language))
		l.texts[path] = p.Source
		l.programs = append(l.programs, placedProgram{key: key, role: role, path: path, program: p})
	}
	if d.Checker != nil {
		place("checker", "checker", "check", model.DraftProgram{Language: d.Checker.Language, Source: d.Checker.Text()})
	}
	if d.Interactor != nil {
		place("interactor", "interactor", "interactor", *d.Interactor)
	}
	for _, list := range []struct {
		role     string
		programs []model.DraftNamedProgram
	}{{"validator", d.Validators}, {"generator", d.Generators}, {"solution", d.Solutions}} {
		for _, p := range list.programs {
			place(p.ID.String(), list.role, p.Name, p.DraftProgram)
		}
	}
	for _, s := range d.Statements {
		for _, section := range model.StatementSections {
			if text := *s.Section(section); text != "" {
				l.texts[gfmt.StatementPath(s.Language, section)] = text + "\n"
			}
		}
	}
	for _, a := range d.Assets {
		l.assets[names.Place(gfmt.AssetFolder, a.Name)] = a.ObjectID
	}
	return l
}

// extension returns the source file extension of a language configuration;
// one the catalog lacks keeps its identifier, and compiling it fails later.
func extension(languages *sandbox.Config, language string) string {
	if lang, ok := languages.Langs[language]; ok {
		return "." + lang.Extensions[0]
	}
	return "." + language
}
