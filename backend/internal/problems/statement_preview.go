package problems

import (
	"context"
	"errors"
	"fmt"
	"io"

	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/brawler2011/gate/backend/pkg/statement"
	"github.com/google/uuid"
)

// PreviewStatement typesets an edited statement of the draft as the site shows it.
func (uc *DraftUseCase) PreviewStatement(ctx context.Context, id uuid.UUID, s model.DraftStatement) (map[string]string, error) {
	if _, err := uc.preview(ctx, id, s); err != nil {
		return nil, err
	}
	html, err := statement.HTML(ctx, uc.Typeset, s.SiteSections())
	return html, previewError(err)
}

// PreviewStatementPDF typesets an edited statement of the draft as builds print it, without samples, which only builds make.
func (uc *DraftUseCase) PreviewStatementPDF(ctx context.Context, id uuid.UUID, s model.DraftStatement) ([]byte, error) {
	d, err := uc.preview(ctx, id, s)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{"olymp.sty": statement.OlympSty}
	for _, a := range model.StatementImages(s, d.Assets) {
		if files["assets/"+a.Name], err = uc.readObject(ctx, a.ObjectID); err != nil {
			return nil, err
		}
	}
	limits := model.PrintedLimits{TimeLimitMs: d.TimeLimitMs, MemoryLimitMb: d.MemoryLimitMb, InputFile: d.InputFile, OutputFile: d.OutputFile}
	doc := statement.NewDocument(statement.Contest{Title: d.Title, Language: s.Language, Problems: []statement.Problem{model.PrintedProblem(d.Title, limits, s, nil)}})
	pdf, err := uc.Typeset.PDF(ctx, doc.Source, files)
	return pdf, previewError(doc.Explain(err))
}

func (uc *DraftUseCase) preview(ctx context.Context, id uuid.UUID, s model.DraftStatement) (model.Draft, error) {
	if _, err := uc.authorize(ctx, id, false); err != nil {
		return model.Draft{}, err
	}
	for _, section := range s.SiteSections() {
		if len(section.Text) > int(uc.DraftQuotas.FileBytes) {
			return model.Draft{}, pkg.ErrPayloadTooLarge
		}
	}
	return uc.Repo.GetDraft(ctx, id)
}

func (uc *DraftUseCase) readObject(ctx context.Context, id uuid.UUID) ([]byte, error) {
	o, err := uc.Repo.GetObject(ctx, id)
	if err != nil {
		return nil, err
	}
	r, _, err := uc.Store.DownloadFile(ctx, uc.Bucket, o.Key, nil)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	data, err := io.ReadAll(io.LimitReader(r, o.Size))
	if err != nil {
		return nil, fmt.Errorf("read object %s: %w", id, err)
	}
	return data, nil
}

func previewError(err error) error {
	if e, ok := errors.AsType[*statement.Error](err); ok {
		return pkg.Invalid(pkg.ErrFormat, e.Error(), map[string]any{"value": e.Section})
	}
	return err
}
