package command

import "context"

type CTXKey struct{}
type CTXValue struct {
	Output *Output
}

func GetFormatFromContext(ctx context.Context) *Output {
	v := ctx.Value(CTXKey{})
	if v == nil {
		return nil
	}
	return v.(*CTXValue).Output
}

func SetFormatInContext(ctx context.Context, output *Output) context.Context {
	return context.WithValue(ctx, CTXKey{}, &CTXValue{Output: output})
}
