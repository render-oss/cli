package command

import (
	"context"

	"github.com/spf13/cobra"
)

type CTXOutputKey struct{}
type CTXOutputValue struct {
	Output *Output
}

func GetFormatFromContext(ctx context.Context) *Output {
	if ctx == nil {
		return nil
	}
	v := ctx.Value(CTXOutputKey{})
	if v == nil {
		return nil
	}
	return v.(*CTXOutputValue).Output
}

func IsInteractive(ctx context.Context) bool {
	output := GetFormatFromContext(ctx)
	return output == nil || *output == Interactive
}

func SetFormatInContext(ctx context.Context, output *Output) context.Context {
	return context.WithValue(ctx, CTXOutputKey{}, &CTXOutputValue{Output: output})
}

type CTXConfirmKey struct{}
type CTXConfirmValue struct {
	Confirm bool
}

func GetConfirmFromContext(ctx context.Context) bool {
	v := ctx.Value(CTXConfirmKey{})
	if v == nil {
		return false
	}
	return v.(*CTXConfirmValue).Confirm
}

func SetConfirmInContext(ctx context.Context, confirm bool) context.Context {
	return context.WithValue(ctx, CTXConfirmKey{}, &CTXConfirmValue{Confirm: confirm})
}

func DefaultFormatNonInteractive(cmd *cobra.Command) {
	format := GetFormatFromContext(cmd.Context())
	if format.Interactive() {
		newFormat := TEXT
		ctx := SetFormatInContext(cmd.Context(), &newFormat)
		cmd.SetContext(ctx)
	}
}
