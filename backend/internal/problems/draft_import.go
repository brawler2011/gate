package problems

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/formats"
	"github.com/brawler2011/gate/backend/pkg/formats/gfmt"
	"github.com/brawler2011/gate/backend/pkg/ziputil"
	"github.com/google/uuid"
)

type importObject struct {
	temporaryID uuid.UUID
	path        string
	spec        model.UploadSpec
}
type draftImport struct {
	draft   model.Draft
	objects []importObject
	root    string
}

func (i *draftImport) Close() { _ = os.RemoveAll(i.root) }
func languageForPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".java":
		return "java"
	}
	return ""
}
func (uc *DraftUseCase) parseImport(ctx context.Context, reader io.Reader, id uuid.UUID, revision int64) (*draftImport, error) {
	root, err := os.MkdirTemp(uc.TempDir, "import-")
	if err != nil {
		return nil, fmt.Errorf("mkdirtemp: %w", err)
	}
	result := &draftImport{root: root}
	complete := false
	defer func() {
		if !complete {
			result.Close()
		}
	}()
	archive, err := uc.spool(ctx, root, "asset", reader)
	if err != nil {
		return nil, err
	}
	defer archive.file.Close()
	z, err := zip.NewReader(archive.file, archive.spec.Size)
	if err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid import ZIP")
	}
	tree := filepath.Join(root, "tree")
	if err := ziputil.ExtractZipLimited(z, tree, int64(uc.BuildOptions.Limits.DiskBytes)-archive.spec.Size, uc.BuildOptions.Limits.FileCount); err != nil {
		return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid import archive")
	}
	packageRoot, err := detectImportRoot(tree)
	if err != nil {
		return nil, err
	}
	// A published v2 package preserves its complete original author snapshot.
	if info, statErr := os.Stat(filepath.Join(packageRoot, ".gate/snapshot.json")); statErr == nil {
		if info.Size() > 16*1024*1024 {
			return nil, pkg.ErrPayloadTooLarge
		}
		data, err := os.ReadFile(filepath.Join(packageRoot, ".gate/snapshot.json"))
		if err != nil {
			return nil, fmt.Errorf("readfile: %w", err)
		}
		var snapshot model.BuildSnapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return nil, fmt.Errorf("unmarshal: %w", err)
		}
		if snapshot.SchemaVersion != model.DraftSnapshotVersion {
			return nil, pkg.ErrBadInput
		}
		result.draft = snapshot.Draft
		result.draft.ProblemID = id
		result.draft.Revision = revision
		for _, oid := range snapshot.Draft.InputObjects() {
			o, err := uc.importObject(filepath.Join(packageRoot, ".gate/objects/"+oid.String()), "asset", oid)
			if err != nil {
				return nil, err
			}
			result.objects = append(result.objects, o)
		}
	} else {
		format, err := formats.DetectFormat(packageRoot)
		if err != nil {
			return nil, pkg.Wrap(pkg.ErrBadInput, err, "unsupported import format")
		}
		parser, err := formats.GetParser(format)
		if err != nil {
			return nil, err
		}
		plan, err := parser.Parse(packageRoot)
		if err != nil {
			return nil, pkg.Wrap(pkg.ErrBadInput, err, "invalid author package")
		}
		if err := uc.convertImport(result, packageRoot, plan, id, revision); err != nil {
			return nil, err
		}
	}
	for path, text := range result.draft.Files {
		if strings.HasPrefix(path, "statements/") {
			statement := ParseStatementMarkdown(text)
			statement.Title = ""
			result.draft.Files[path] = RenderStatementMarkdown(statement)
		}
	}
	if err := result.draft.ComputeScores(); err != nil {
		return nil, err
	}
	complete = true
	return result, nil
}
func detectImportRoot(root string) (string, error) {
	if _, err := formats.DetectFormat(root); err == nil {
		return root, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("readdir: %w", err)
	}
	found := ""
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if _, err := formats.DetectFormat(path); err == nil {
			if found != "" {
				return "", pkg.Wrap(pkg.ErrBadInput, nil, "multiple packages in archive")
			}
			found = path
		}
	}
	if found == "" {
		return "", pkg.Wrap(pkg.ErrBadInput, nil, "package root not found")
	}
	return found, nil
}
func (uc *DraftUseCase) importObject(path, purpose string, id uuid.UUID) (importObject, error) {
	f, err := os.Open(path)
	if err != nil {
		return importObject{}, fmt.Errorf("open: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return importObject{}, fmt.Errorf("stat: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > uc.ObjectBytes {
		return importObject{}, pkg.ErrPayloadTooLarge
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return importObject{}, fmt.Errorf("copy: %w", err)
	}
	return importObject{temporaryID: id, path: path, spec: model.UploadSpec{Purpose: purpose, SHA256: hex.EncodeToString(h.Sum(nil)), Size: n}}, nil
}
func (uc *DraftUseCase) convertImport(result *draftImport, root string, plan *gfmt.ImportPlan, id uuid.UUID, revision int64) error {
	p := plan.Problem
	d := model.Draft{ProblemID: id, Revision: revision, Title: p.Title, ShortName: "problem-" + id.String()[:8], Type: p.Type, TimeLimitMs: p.Limits.TimeMs, MemoryLimitMb: p.Limits.MemoryMb, Files: map[string]string{}, Components: []model.DraftComponent{}, Tests: []model.DraftTest{}, Subtasks: []model.DraftSubtask{}, Assets: []model.DraftAsset{}}
	paths := map[string]string{}
	for _, mapping := range plan.Mappings {
		if !fs.ValidPath(mapping.SourcePath) || !model.ValidDraftPath(mapping.TargetPath) {
			return pkg.ErrBadInput
		}
		if _, ok := paths[mapping.TargetPath]; ok {
			return pkg.ErrBadInput
		}
		paths[mapping.TargetPath] = filepath.Join(root, mapping.SourcePath)
	}
	names := make([]string, 0, len(paths))
	for path := range paths {
		names = append(names, path)
	}
	sort.Strings(names)
	componentIDs := map[string]uuid.UUID{}
	generatorIDs := map[string]uuid.UUID{}
	for _, path := range names {
		role := model.ComponentRole(path)
		if role != "" || strings.HasPrefix(path, model.LibraryFolder+"/") || strings.HasPrefix(path, model.StatementFolder+"/") {
			info, err := os.Stat(paths[path])
			if err != nil {
				return fmt.Errorf("stat: %w", err)
			}
			if info.Size() > int64(uc.DraftQuotas.FileBytes) {
				return pkg.ErrPayloadTooLarge
			}
			data, err := os.ReadFile(paths[path])
			if err != nil {
				return fmt.Errorf("readfile: %w", err)
			}
			if !utf8.Valid(data) {
				return pkg.Wrap(pkg.ErrBadInput, nil, "source text must be UTF-8")
			}
			d.Files[path] = string(data)
			if role != "" {
				cid := uuid.New()
				componentIDs[path] = cid
				language := p.Sources[path].Compiler
				if language == "" {
					language = languageForPath(path)
				}
				if _, ok := uc.BuildOptions.Toolchain.Languages[language]; !ok {
					return pkg.Wrap(pkg.ErrBadInput, nil, "unsupported imported source language")
				}
				deps := map[string]string{}
				for destination, source := range p.Sources[path].DependencyMap {
					deps[destination] = source
				}
				for _, source := range p.Sources[path].Dependencies {
					if !strings.HasPrefix(source, "lib/") {
						source = "lib/" + source
					}
					deps[strings.TrimPrefix(source, "lib/")] = source
				}
				if len(deps) == 0 {
					for _, source := range names {
						if strings.HasPrefix(source, "lib/") {
							deps[strings.TrimPrefix(source, "lib/")] = source
						}
					}
				}
				d.Components = append(d.Components, model.DraftComponent{ID: cid, Role: role, Path: path, Language: language, Dependencies: deps})
				if role == "generator" {
					// Tests name generators by stem, so the stem must identify one file.
					name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
					if _, ok := generatorIDs[name]; ok {
						return pkg.Wrap(pkg.ErrBadInput, nil, "several generators named "+name)
					}
					generatorIDs[name] = cid
				}
			}
		}
	}
	selection := func(path, dir string) *uuid.UUID {
		if path == "" {
			return nil
		}
		if !strings.HasPrefix(path, dir+"/") {
			path = dir + "/" + path
		}
		id, ok := componentIDs[path]
		if !ok {
			return nil
		}
		return &id
	}
	d.CheckerID = selection(p.Checker, "checkers")
	d.ValidatorID = selection(p.Validator, "validators")
	d.InteractorID = selection(p.Interactor, "interactors")
	d.MainSolutionID = selection(p.Solutions["main"], "solutions")
	if d.MainSolutionID == nil {
		for _, path := range names {
			if p.Solutions[strings.TrimPrefix(path, "solutions/")] == "OK" {
				d.MainSolutionID = selection(path, "solutions")
				break
			}
		}
	}
	objectIDs := map[string]uuid.UUID{}
	addObject := func(path, purpose string) (*uuid.UUID, error) {
		if oid, ok := objectIDs[path]; ok {
			return &oid, nil
		}
		actual, ok := paths[path]
		if !ok {
			return nil, pkg.Wrap(pkg.ErrBadInput, nil, "missing imported file "+path)
		}
		oid := uuid.New()
		object, err := uc.importObject(actual, purpose, oid)
		if err != nil {
			return nil, err
		}
		result.objects = append(result.objects, object)
		objectIDs[path] = oid
		return &oid, nil
	}
	groupIDs := map[string]uuid.UUID{}
	for _, name := range p.OrderedSubtaskNames() {
		groupIDs[name] = uuid.New()
	}
	sampleKeys := map[string]bool{}
	normalKeys := map[string]bool{}
	testKey := func(t gfmt.Test) string {
		if t.Manual != "" {
			return "manual:" + t.Manual
		}
		return "generator:" + t.Generate
	}
	for name, group := range p.Subtasks {
		for _, test := range group.Tests {
			if test.Sample || name == "samples" {
				sampleKeys[testKey(test)] = true
			}
			if name != "samples" {
				normalKeys[testKey(test)] = true
			}
		}
	}
	for _, name := range p.OrderedSubtaskNames() {
		group := p.Subtasks[name]
		subtask := model.DraftSubtask{ID: groupIDs[name], Ordinal: len(d.Subtasks) + 1, Name: name, Policy: group.Policy, DependsOn: []uuid.UUID{}}
		for _, dep := range group.Dependencies {
			did, ok := groupIDs[dep]
			if !ok {
				return pkg.ErrBadInput
			}
			subtask.DependsOn = append(subtask.DependsOn, did)
		}
		for _, test := range group.Tests {
			if name == "samples" && normalKeys[testKey(test)] {
				continue
			}
			t := model.DraftTest{ID: uuid.New(), Ordinal: len(d.Tests) + 1, SubtaskID: subtask.ID, Points: test.Points, Sample: sampleKeys[testKey(test)]}
			if test.Manual != "" {
				path := test.Manual
				if !strings.HasPrefix(path, "tests/") {
					path = "tests/" + path
				}
				oid, err := addObject(path, "test-input")
				if err != nil {
					return err
				}
				t.Input = model.DraftInput{Mode: "manual", ObjectID: oid, Args: []string{}}
			} else {
				argv, err := parseGeneratorArgv(test.Generate)
				if err != nil || len(argv) == 0 {
					return pkg.Wrap(pkg.ErrBadInput, err, "invalid generator recipe")
				}
				gen, ok := generatorIDs[argv[0]]
				if !ok {
					return pkg.Wrap(pkg.ErrBadInput, nil, "imported generator not found")
				}
				seed := int64(0)
				t.Input = model.DraftInput{Mode: "generator", ComponentID: &gen, Args: argv[1:], Seed: &seed}
			}
			if d.Type == "interactive" {
				t.Answer = model.DraftAnswer{Mode: "interactive"}
			} else {
				answerPath := test.Answer
				if answerPath == "" && test.Manual != "" {
					base := strings.TrimPrefix(test.Manual, "tests/")
					for _, candidate := range gfmt.TestOutputNames(base) {
						path := "tests/" + candidate
						if _, ok := paths[path]; ok {
							answerPath = path
							break
						}
					}
				}
				if answerPath != "" {
					if !strings.HasPrefix(answerPath, "tests/") {
						answerPath = "tests/" + answerPath
					}
					oid, err := addObject(answerPath, "test-answer")
					if err != nil {
						return err
					}
					t.Answer = model.DraftAnswer{Mode: "provided", ObjectID: oid}
				} else {
					t.Answer = model.DraftAnswer{Mode: "generated", ComponentID: d.MainSolutionID}
				}
			}
			d.Tests = append(d.Tests, t)
		}
		if name == "samples" && len(group.Tests) > 0 && len(d.Tests) == 0 {
			continue
		}
		d.Subtasks = append(d.Subtasks, subtask)
	}
	for _, path := range names {
		if _, ok := d.Files[path]; ok || strings.HasPrefix(path, "tests/") {
			continue
		}
		if !strings.HasPrefix(path, model.AssetFolder+"/") {
			return pkg.Wrap(pkg.ErrBadInput, nil, "unexpected file "+path)
		}
		oid, err := addObject(path, "asset")
		if err != nil {
			return err
		}
		d.Assets = append(d.Assets, model.DraftAsset{Path: path, ObjectID: *oid})
	}
	result.draft = d
	return nil
}
func parseGeneratorArgv(command string) ([]string, error) {
	// Only argument quoting/escaping is parsed. Redirection is presentation syntax;
	// execution always uses argv, never a shell or expansion.
	command = strings.TrimSpace(command)
	if strings.HasSuffix(command, "> $") {
		command = strings.TrimSpace(strings.TrimSuffix(command, "> $"))
	}
	args := []string{}
	var current strings.Builder
	var quote rune
	escaped, started := false, false
	for _, r := range command {
		if escaped {
			current.WriteRune(r)
			escaped = false
			started = true
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			started = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			started = true
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			started = true
			continue
		}
		if r == ' ' || r == '\t' {
			if started {
				args = append(args, current.String())
				current.Reset()
				started = false
			}
			continue
		}
		if strings.ContainsRune("|;&><`$\n\r", r) {
			return nil, fmt.Errorf("shell syntax is not supported in generator arguments")
		}
		current.WriteRune(r)
		started = true
	}
	if quote != 0 || escaped {
		return nil, fmt.Errorf("unterminated generator argument")
	}
	if started {
		args = append(args, current.String())
	}
	return args, nil
}
func (i *draftImport) bind(ids []uuid.UUID) error {
	if len(ids) != len(i.objects) {
		return pkg.ErrConflict
	}
	replace := map[uuid.UUID]uuid.UUID{}
	for n, o := range i.objects {
		replace[o.temporaryID] = ids[n]
	}
	for n := range i.draft.Tests {
		t := &i.draft.Tests[n]
		if t.Input.ObjectID != nil {
			id, ok := replace[*t.Input.ObjectID]
			if !ok {
				return pkg.ErrBadInput
			}
			t.Input.ObjectID = &id
		}
		if t.Answer.ObjectID != nil {
			id, ok := replace[*t.Answer.ObjectID]
			if !ok {
				return pkg.ErrBadInput
			}
			t.Answer.ObjectID = &id
		}
	}
	for n := range i.draft.Assets {
		id, ok := replace[i.draft.Assets[n].ObjectID]
		if !ok {
			return pkg.ErrBadInput
		}
		i.draft.Assets[n].ObjectID = id
	}
	return nil
}
