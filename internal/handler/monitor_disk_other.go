//go:build !windows

package handler

func diskTotalGB() float64 { return 0 }
func diskFreeGB() float64  { return 0 }
