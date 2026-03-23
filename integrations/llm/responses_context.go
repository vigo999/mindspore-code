package llm

import "context"

type responsesContextKey string

const previousResponseIDKey responsesContextKey = "openai_responses_previous_response_id"

// WithPreviousResponseID annotates a request context for task-local Responses API chaining.
func WithPreviousResponseID(ctx context.Context, responseID string) context.Context {
	if ctx == nil || responseID == "" {
		return ctx
	}
	return context.WithValue(ctx, previousResponseIDKey, responseID)
}

// PreviousResponseIDFromContext returns the task-local Responses API parent response ID.
func PreviousResponseIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	responseID, _ := ctx.Value(previousResponseIDKey).(string)
	return responseID
}
