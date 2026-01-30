package pkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gate149/pandoc-go-sdk"
)

type Client struct {
	client *pandoc.ClientWithResponses
}

func NewPandocClient(httpClient *http.Client, address string) *Client {
	c, err := pandoc.NewClientWithResponses(address, pandoc.WithHTTPClient(httpClient))
	if err != nil {
		panic(fmt.Errorf("failed to create pandoc client: %w", err))
	}
	return &Client{
		client: c,
	}
}

func (c *Client) ConvertLatexToHtml5(ctx context.Context, text string) (string, error) {
	from := "latex"
	to := "html5"
	math := pandoc.Katex

	body := pandoc.ConversionRequest{
		From:           &from,
		To:             &to,
		Text:           text,
		HtmlMathMethod: &math,
	}

	resp, err := c.client.PostWithResponse(ctx, body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode() != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	var errResp pandoc.ErrorResponse
	if err := json.Unmarshal(resp.Body, &errResp); err == nil && errResp.Error != "" {
		return "", errors.New(errResp.Error)
	}

	var successResp pandoc.SuccessResponse
	if err := json.Unmarshal(resp.Body, &successResp); err == nil {
		return successResp.Output, nil
	}

	return "", errors.New("failed to parse response")
}

func (c *Client) BatchConvertLatexToHtml5(ctx context.Context, texts []string) ([]string, error) {
	from := "latex"
	to := "html5"
	math := pandoc.Katex

	reqs := make([]pandoc.ConversionRequest, len(texts))
	for i, text := range texts {
		reqs[i] = pandoc.ConversionRequest{
			From:           &from,
			To:             &to,
			Text:           text,
			HtmlMathMethod: &math,
		}
	}

	resp, err := c.client.PostBatchWithResponse(ctx, reqs)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	var results []json.RawMessage
	if err := json.Unmarshal(resp.Body, &results); err != nil {
		return nil, fmt.Errorf("failed to parse batch response: %w", err)
	}

	if len(results) != len(texts) {
		return nil, fmt.Errorf("wrong number of fields returned: %d", len(results))
	}

	outputs := make([]string, len(texts))
	var errs error

	for i, raw := range results {
		var errResp pandoc.ErrorResponse
		if err := json.Unmarshal(raw, &errResp); err == nil && errResp.Error != "" {
			errs = errors.Join(errs, errors.New(errResp.Error))
			continue
		}

		var successResp pandoc.SuccessResponse
		if err := json.Unmarshal(raw, &successResp); err == nil {
			outputs[i] = successResp.Output
			continue
		}

		errs = errors.Join(errs, errors.New("failed to parse item response"))
	}

	if errs != nil {
		return nil, Wrap(ErrBadInput, errs, "invalid input")
	}

	return outputs, nil
}
