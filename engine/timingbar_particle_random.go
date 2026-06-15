package engine

import "math"

func timingStarSeed(h timingHit, idx int) uint32 {
	bits := math.Float64bits(h.t + h.y*17)
	return uint32(bits) ^ uint32(bits>>32) ^ uint32(idx*0x9e3779b9) ^ uint32(h.rating)*0x85ebca6b
}

func timingRand(seed uint32, salt int) float64 {
	x := seed + uint32(salt)*0x9e3779b9
	x ^= x >> 16
	x *= 0x7feb352d
	x ^= x >> 15
	x *= 0x846ca68b
	x ^= x >> 16
	return float64(x&0xffffff) / float64(0x1000000)
}
