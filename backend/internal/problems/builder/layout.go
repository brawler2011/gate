package builder

import (
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/google/uuid"
)

// placedProgram is a draft program at its package path. Key identifies it
// in the build: the role of a single checker or interactor, else its ID.
type placedProgram struct {
	key     string
	role    string
	path    string
	program model.DraftProgram
	// dependencies maps each include name to the package path of its file.
	dependencies map[string]string
}

// packageLayout is a draft placed in the Gate package layout.
type packageLayout struct {
	texts    map[string]string
	programs []placedProgram
	assets   map[string]uuid.UUID
}

// placeDraft decides where each draft entry goes in the built package. Names
// are labels, so entries whose file names collide are numbered.
func placeDraft(d model.Draft) packageLayout {
	l := packageLayout{texts: map[string]string{}, assets: map[string]uuid.UUID{}}
	names := gfmt.FileNames{}
	library := map[uuid.UUID]string{}
	for _, f := range d.Library {
		library[f.ID] = names.Place(gfmt.LibraryFolder, f.Name)
		l.texts[library[f.ID]] = f.Content
	}
	place := func(key, role, name string, p model.DraftProgram) {
		path := names.Place(gfmt.RoleFolder(role), name+gfmt.Extension(p.Language))
		deps := map[string]string{}
		for _, id := range p.Dependencies {
			deps[d.LibraryFile(id).Name] = library[id]
		}
		l.texts[path] = p.Source
		l.programs = append(l.programs, placedProgram{key: key, role: role, path: path, program: p, dependencies: deps})
	}
	if d.Checker != nil {
		place("checker", "checker", "check", *d.Checker)
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
