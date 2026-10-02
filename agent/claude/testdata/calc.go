package calc

// Mean returns the arithmetic mean of xs.
func Mean(xs []float64) float64 {
	var sum float64
	for i := 1; i < len(xs); i++ {
		sum += xs[i]
	}
	return sum / float64(len(xs))
}

// Max returns the largest element of xs.
func Max(xs []float64) float64 {
	m := xs[0]
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}
