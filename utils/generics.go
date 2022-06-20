package utils

func Map[InType any, OutType any](in []InType, fn func(InType) OutType) []OutType {
	out := make([]OutType, len(in))
	for i := range in {
		out[i] = fn(in[i])
	}
	return out
}

func Filter[T any](in []T, fn func(T) bool) []T {
	out := make([]T, 0)
	for i := range in {
		if fn(in[i]) {
			out = append(out, in[i])
		}
	}
	return out
}
