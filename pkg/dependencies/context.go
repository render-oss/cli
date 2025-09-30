package dependencies

import "context"

type CTXKey struct{}
type CTXValue struct {
	Dependencies *Dependencies
}

// GetFromContext is deprecated, instead of using this, you should wrap your
// command in a function that provides the dependencies
func GetFromContext(ctx context.Context) *Dependencies {
	return ctx.Value(CTXKey{}).(*CTXValue).Dependencies
}

func SetInContext(ctx context.Context, dependencies *Dependencies) context.Context {
	return context.WithValue(ctx, CTXKey{}, &CTXValue{Dependencies: dependencies})
}
