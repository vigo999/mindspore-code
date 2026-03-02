package domain

import "context"

type ModelClient interface {
	Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, error)
}

type Factory interface {
	ClientFor(spec ModelSpec) (ModelClient, error)
	Providers() []ProviderInfo
}
