package builder

import (
	"github.com/brawler2011/gate/backend/internal/problems/archive"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg/sandbox"
)

type placedProgram struct {
	key     string
	role    string
	path    string
	program model.DraftProgram
}

func programKey(role, name string) string { return role + "/" + name }

func programs(d model.Draft, languages *sandbox.Config) []placedProgram {
	var result []placedProgram
	place := func(key, role, name string, p model.DraftProgram) {
		result = append(result, placedProgram{key: key, role: role, path: archive.ProgramPath(languages, role, name, p.Language), program: p})
	}
	if d.Checker != nil {
		place("checker", "checker", "checker", model.DraftProgram{Language: d.Checker.Language, Source: d.Checker.Text()})
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
	return result
}
