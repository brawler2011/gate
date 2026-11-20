package builder

import (
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
	"github.com/google/uuid"
)

type placedProgram struct {
	key     string
	role    string
	path    string
	program model.DraftProgram
}

type packageLayout struct {
	texts    map[string]string
	programs []placedProgram
	assets   map[string]uuid.UUID
}

func programKey(role, name string) string { return role + "/" + name }

func placeDraft(d model.Draft, languages *sandbox.Config) packageLayout {
	l := packageLayout{texts: map[string]string{}, assets: map[string]uuid.UUID{}}
	for _, f := range d.Library {
		l.texts[gfmt.LibraryFolder+"/"+f.Name] = f.Text()
	}
	place := func(key, role, name string, p model.DraftProgram) {
		path := gfmt.RoleFolder(role) + "/" + name + extension(languages, p.Language)
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
	}{{"validator", d.ValidatorPrograms()}, {"generator", d.Generators}, {"solution", d.SolutionPrograms()}} {
		for _, p := range list.programs {
			place(programKey(list.role, p.Name), list.role, p.Name, p.DraftProgram)
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
		l.assets[gfmt.AssetFolder+"/"+a.Name] = a.ObjectID
	}
	return l
}

// Unknown languages retain their identifiers and fail at compilation.
func extension(languages *sandbox.Config, language string) string {
	if lang, ok := languages.Langs[language]; ok {
		return "." + lang.Extensions[0]
	}
	return "." + language
}
