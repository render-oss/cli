package tui

import "context"

type CTXKey struct{}
type CTXValue struct {
	Stack *StackModel
}

func GetStackFromContext(ctx context.Context) *StackModel {
	v := ctx.Value(CTXKey{})
	if v == nil {
		return nil
	}
	return v.(*CTXValue).Stack
}

func SetStackInContext(ctx context.Context, stack *StackModel) context.Context {
	return context.WithValue(ctx, CTXKey{}, &CTXValue{Stack: stack})
}
