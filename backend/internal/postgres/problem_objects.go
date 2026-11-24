package postgres

import (
	"context"
	"encoding/json"
	"time"

	sqlc "github.com/brawler2011/gate/backend/internal/postgres/sqlc"
	model "github.com/brawler2011/gate/backend/internal/problems/model"
	"github.com/brawler2011/gate/backend/pkg"
	"github.com/google/uuid"
)

func mapStoredObject(o sqlc.StoredObject) model.StoredObject {
	return model.StoredObject{ID: o.ID, OrganizationID: o.OrganizationID, Purpose: o.Purpose, Key: o.Key, SHA256: o.Sha256, Size: o.Size}
}
func (r *ProblemDraftsRepo) StoredObjects(ctx context.Context, ids []uuid.UUID) ([]model.StoredObject, error) {
	rows, err := sqlc.New(r.pool).ListStoredObjects(ctx, ids)
	if err != nil {
		return nil, dbError("list build inputs", err)
	}
	objects := make([]model.StoredObject, 0, len(rows))
	for _, o := range rows {
		objects = append(objects, mapStoredObject(o))
	}
	return objects, nil
}

// The caller must store the archive with a verified SHA-256 before publication.
// Publish object and package together so losing a lease leaves only unreferenced bytes.
func (r *ProblemDraftsRepo) PublishBuild(ctx context.Context, id, token uuid.UUID, object model.StoredObject, index model.PackageIndex) (uuid.UUID, error) {
	encode := func(v any) []byte {
		data, _ := json.Marshal(v)
		return data
	}
	if index.Samples == nil {
		index.Samples = []model.Sample{}
	}
	if index.Statements == nil {
		index.Statements = []model.BuiltStatement{}
	}
	objects := []model.StoredObject{object}
	type pdf struct {
		Language string    `json:"language"`
		ObjectID uuid.UUID `json:"object_id"`
	}
	pdfs := []pdf{}
	for _, p := range index.PDFs {
		if p.Object.OrganizationID != object.OrganizationID || p.Object.Purpose != "statement" {
			return uuid.Nil, pkg.ErrInvalid
		}
		objects = append(objects, p.Object)
		pdfs = append(pdfs, pdf{p.Language, p.Object.ID})
	}
	if index.Assets == nil {
		index.Assets = []model.DraftAsset{}
	}
	var inputs *uuid.UUID
	if index.Inputs != nil {
		if index.Inputs.OrganizationID != object.OrganizationID || index.Inputs.Purpose != "inputs" {
			return uuid.Nil, pkg.ErrInvalid
		}
		objects = append(objects, *index.Inputs)
		inputs = &index.Inputs.ID
	}
	tx, err := beginReadCommitted(ctx, r.pool)
	if err != nil {
		return uuid.Nil, err
	}
	defer rollback(ctx, tx)
	q := sqlc.New(tx)
	b, err := q.LockProblemBuild(ctx, id)
	if err != nil {
		return uuid.Nil, dbError("publish build", err)
	}
	if b.State == "succeeded" {
		pid, err := q.BuildPackageID(ctx, id)
		if err != nil {
			return uuid.Nil, dbError("publish build", err)
		}
		return pid, nil
	}
	if object.OrganizationID != b.OrganizationID || object.Purpose != "package" {
		return uuid.Nil, pkg.ErrInvalid
	}
	// The lease check and the transition are one statement, so a lease expiring mid-publication publishes nothing.
	succeeded, err := q.MarkBuildSucceeded(ctx, sqlc.MarkBuildSucceededParams{ID: id, LeaseToken: &token})
	if err != nil {
		return uuid.Nil, dbError("finish build", err)
	}
	if succeeded != 1 {
		return uuid.Nil, pkg.ErrConflict
	}
	if err := insertObjects(ctx, tx, objects); err != nil {
		return uuid.Nil, err
	}
	pid := uuid.New()
	if err := q.InsertProblemPackage(ctx, sqlc.InsertProblemPackageParams{
		ID: pid, ProblemID: b.ProblemID, OrganizationID: b.OrganizationID, BuildID: id, ObjectID: object.ID, PackageHash: object.SHA256, Revision: b.Revision,
		Title: index.Title, ShortName: index.ShortName, Type: index.Type, TimeLimitMs: safeInt32(index.TimeLimitMs), MemoryLimitMb: safeInt32(index.MemoryLimitMb),
		InputFile: index.InputFile, OutputFile: index.OutputFile, Statements: encode(index.Statements), Samples: encode(index.Samples), InputsObjectID: inputs,
	}); err != nil {
		return uuid.Nil, dbError("insert package", err)
	}
	if err := q.InsertPackageAssets(ctx, sqlc.InsertPackageAssetsParams{PackageID: pid, Assets: encode(index.Assets)}); err != nil {
		return uuid.Nil, dbError("insert package assets", err)
	}
	if err := q.InsertPackagePDFs(ctx, sqlc.InsertPackagePDFsParams{PackageID: pid, Pdfs: encode(pdfs)}); err != nil {
		return uuid.Nil, dbError("insert package PDFs", err)
	}
	if err := commitTx(ctx, tx); err != nil {
		resolveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		durable, readErr := sqlc.New(r.pool).SucceededBuildPackageID(resolveCtx, id)
		if readErr == nil {
			return durable, nil
		}
		return uuid.Nil, err
	}
	return pid, nil
}

func (r *ProblemDraftsRepo) GetObject(ctx context.Context, id uuid.UUID) (model.StoredObject, error) {
	o, err := sqlc.New(r.pool).ReadStoredObject(ctx, id)
	if err != nil {
		return model.StoredObject{}, dbError("get object", err)
	}
	return mapStoredObject(o), nil
}

func (r *ProblemDraftsRepo) GetPackageDescriptor(ctx context.Context, id uuid.UUID) (model.PackageDescriptor, error) {
	row, err := sqlc.New(r.pool).ReadPackageDescriptor(ctx, id)
	if err != nil {
		return model.PackageDescriptor{}, dbError("get package descriptor", err)
	}
	p := model.PackageDescriptor{ID: row.ID, ProblemID: row.ProblemID}
	p.Object, err = r.GetObject(ctx, row.ObjectID)
	return p, err
}

func (r *ProblemDraftsRepo) GetPackageStatement(ctx context.Context, id uuid.UUID) (model.PackageStatement, error) {
	row, err := sqlc.New(r.pool).ReadPackageStatement(ctx, id)
	if err != nil {
		return model.PackageStatement{}, dbError("get package statement", err)
	}
	p := model.PackageStatement{ProblemID: row.ProblemID, Title: row.Title}
	if err := json.Unmarshal(row.Statements, &p.Statements); err != nil {
		return p, dbError("decode package statements", err)
	}
	if err := json.Unmarshal(row.Samples, &p.Samples); err != nil {
		return p, dbError("decode package samples", err)
	}
	if err := json.Unmarshal(row.Assets, &p.Assets); err != nil {
		return p, dbError("decode package assets", err)
	}
	return p, nil
}

func (r *ProblemDraftsRepo) FindPackageAssets(ctx context.Context, id uuid.UUID, sha256 string) ([]model.PackageAsset, error) {
	rows, err := sqlc.New(r.pool).ListPackageAssetsBySHA256(ctx, sqlc.ListPackageAssetsBySHA256Params{PackageID: id, Sha256: sha256})
	if err != nil {
		return nil, dbError("list package assets", err)
	}
	assets := make([]model.PackageAsset, 0, len(rows))
	for _, row := range rows {
		assets = append(assets, model.PackageAsset{Name: row.Name, Key: row.Key})
	}
	return assets, nil
}

func (r *ProblemDraftsRepo) FindPackageInputs(ctx context.Context, id uuid.UUID) (string, error) {
	key, err := sqlc.New(r.pool).FindPackageInputs(ctx, id)
	if err != nil {
		return "", dbError("find package inputs", err)
	}
	return key, nil
}

func (r *ProblemDraftsRepo) FindPackagePDF(ctx context.Context, id uuid.UUID, language string) (string, error) {
	key, err := sqlc.New(r.pool).FindPackagePDF(ctx, sqlc.FindPackagePDFParams{PackageID: id, Language: language})
	if err != nil {
		return "", dbError("find package PDF", err)
	}
	return key, nil
}
