package generics

import (
	"context"
)

func ForEach[S ~[]T, T any](in S, fn func(T)) {
	for _, x := range in {
		fn(x)
	}
}

func ForEachWithError[S ~[]T, T any](in S, fn func(T) error) error {
	for _, x := range in {
		err := fn(x)
		if err != nil {
			return err
		}
	}
	return nil
}

func Map[S []InType, U []OutType, InType any, OutType any](in S, fn func(InType) OutType) U {
	out := make([]OutType, len(in))
	for i := range in {
		out[i] = fn(in[i])
	}
	return out
}

func MapWithError[S []InType, U []OutType, InType any, OutType any](in S, fn func(InType) (OutType, error)) (U, error) {
	out := make([]OutType, len(in))
	var err error
	for i := range in {
		out[i], err = fn(in[i])
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func FlatMap[S []InType, U []OutType, InType any, OutType any](in S, fn func(InType) []OutType) U {
	var out U
	for i := range in {
		for _, x := range fn(in[i]) {
			out = append(out, x)
		}
	}
	return out
}

func Reduce[S ~[]InType, InType any, OutType any](in S, fn func(OutType, InType) OutType, out OutType) OutType {
	for i := range in {
		out = fn(out, in[i])
	}
	return out
}

func Flatten[S ~[]T, T any](in [][]T) []T {
	var out []T
	for _, xs := range in {
		for _, x := range xs {
			out = append(out, x)
		}
	}
	return out
}

func Filter[S ~[]T, T any](in S, fn func(T) bool) []T {
	out := make([]T, 0)
	for i := range in {
		if fn(in[i]) {
			out = append(out, in[i])
		}
	}
	return out
}

func AnyFunc[T any](in []T, fn func(in T) bool) bool {
	for i := range in {
		if fn(in[i]) {
			return true
		}
	}
	return false
}

func First[S ~[]T, T any, U any](in S, fn func(T) (U, bool)) (u U, ok bool) {
	for i := range in {
		u, ok := fn(in[i])
		if ok {
			return u, true
		}
	}
	return u, false
}

func Max[T ordered](in ...T) (t T, ok bool) {
	if len(in) == 0 {
		return
	}
	max := in[0]
	for i := 1; i < len(in); i++ {
		if in[i] > max {
			max = in[i]
		}
	}
	return max, true
}

func MaxFunc[S ~[]T, T any, N ordered](in S, fn func(T) N) (t T, ok bool) {
	if len(in) == 0 {
		return
	}
	var max N
	maxIdx := 0
	for i := range in {
		if next := fn(in[i]); next > max {
			max = next
			maxIdx = i
		}
	}
	return in[maxIdx], true
}

func Min[T ordered](in ...T) (t T, ok bool) {
	if len(in) == 0 {
		return
	}
	min := in[0]
	for i := 1; i < len(in); i++ {
		if in[i] < min {
			min = in[i]
		}
	}
	return min, true
}

func MinFunc[S ~[]T, T any, N ordered](in S, fn func(T) N) (t T, ok bool) {
	if len(in) == 0 {
		return
	}
	var min N
	minIdx := 0
	for i := range in {
		if next := fn(in[i]); next < min {
			min = next
			minIdx = i
		}
	}
	return in[minIdx], true
}

func Contains[S ~[]T, T comparable](in S, item T) bool {
	for _, x := range in {
		if x == item {
			return true
		}
	}
	return false
}

func Reverse[S ~[]T, T any](in S) []T {
	out := make([]T, len(in))
	i := len(in) - 1
	j := 0
	for i >= 0 {
		out[i] = in[j]
		i--
		j++
	}
	return out
}

func Keys[M ~map[K]V, K comparable, V any](m M) []K {
	keys := make([]K, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	return keys
}

func Values[M ~map[K]V, K comparable, V any](m M) []V {
	values := make([]V, len(m))
	i := 0
	for _, v := range m {
		values[i] = v
		i++
	}
	return values
}

type Tuple2[A any, B any] struct {
	A A
	B B
}

func Zip[A any, B any](a []A, b []B) []Tuple2[A, B] {
	return ZipFunc(a, b, func(a A, b B, _ int) Tuple2[A, B] { return Tuple2[A, B]{a, b} })
}

func ZipFunc[A any, B any, C any](a []A, b []B, fn func(A, B, int) C) []C {
	length, ok := Min(len(a), len(b))
	if !ok {
		return nil
	}
	zipped := make([]C, length)
	for i := 0; i < length; i++ {
		zipped[i] = fn(a[i], b[i], i)
	}
	return zipped
}

func ZipFunc3[A any, B any, C any, D any](a []A, b []B, c []C, fn func(A, B, C, int) D) []D {
	length, ok := Min(len(a), len(b), len(c))
	if !ok {
		return nil
	}
	zipped := make([]D, length)
	for i := 0; i < length; i++ {
		zipped[i] = fn(a[i], b[i], c[i], i)
	}
	return zipped
}

func UnzipFunc3[A any, B any, C any, D any](a []A, fn func(A, int) (B, C, D)) ([]B, []C, []D) {
	if len(a) == 0 {
		return nil, nil, nil
	}
	bs := make([]B, len(a))
	cs := make([]C, len(a))
	ds := make([]D, len(a))
	for i := 0; i < len(a); i++ {
		bs[i], cs[i], ds[i] = fn(a[i], i)
	}
	return bs, cs, ds
}

type ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

func MapChan[T any, U any](ctx context.Context, ch <-chan T, fn func(t T) U) <-chan U {
	chOut := make(chan U)
	go func() {
		defer close(chOut)
		for {
			select {
			case <-ctx.Done():
				return
			case t, open := <-ch:
				if !open {
					return
				}

				select {
				case <-ctx.Done():
					return
				case chOut <- fn(t):
				}
			}
		}
	}()
	return chOut
}
