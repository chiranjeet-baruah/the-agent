package openaicompat_outbound_adapter

import (
	"context"
	"time"
)

// ListModels returns the model IDs a backend currently exposes. A successful
// call also proves the base URL/API key pair is reachable and authenticates
// — used at startup to discover which of a provider's configured models are
// actually available before picking one.
func ListModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	page, err := newClient(baseURL, apiKey).Models.List(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(page.Data))
	for _, m := range page.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}
