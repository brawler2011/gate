package interfaces

import "context"

type PandocClient interface {
	BatchConvertLatexToHtml5(ctx context.Context, texts []string) ([]string, error)
}
