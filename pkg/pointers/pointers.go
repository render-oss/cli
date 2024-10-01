package pointers

func From[T any](x T) *T {
	return &x
}

func FromArray[T any](x []T) *[]T {
	if len(x) == 0 {
		return nil
	}

	return &x
}
