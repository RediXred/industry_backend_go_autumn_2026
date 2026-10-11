package main

import "math"

type Stats struct {
	Count         int
	Sum, Min, Max int64
}

func Calc(nums []int64) Stats {
	if len(nums) < 2 {
		return Stats{}
	}

	stats := Stats{
		Min: math.MaxInt64,
		Max: math.MinInt64,
	}

	for i := 1; i < len(nums); i++ {
		r := nums[i] - nums[i-1]
		stats.Count++
		stats.Sum += r
		if r > stats.Max {
			stats.Max = r
		}
		if r < stats.Min {
			stats.Min = r
		}
	}

	return stats
}
