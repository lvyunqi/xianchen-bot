package util

import (
	"crypto/rand"
	"math/big"
)

func Range(minimum, maximum int) int {
	if maximum <= minimum {
		return minimum
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(maximum-minimum+1)))
	if err != nil {
		return minimum
	}
	return minimum + int(v.Int64())
}

type Weighted[T any] struct {
	Value  T
	Weight int
}

func WeightedPick[T any](values []Weighted[T]) (T, bool) {
	var zero T
	total := 0
	for _, v := range values {
		if v.Weight > 0 {
			total += v.Weight
		}
	}
	if total == 0 {
		return zero, false
	}
	roll := Range(1, total)
	for _, v := range values {
		roll -= v.Weight
		if roll <= 0 {
			return v.Value, true
		}
	}
	return zero, false
}
