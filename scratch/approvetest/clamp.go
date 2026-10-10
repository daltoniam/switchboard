package scratch

// Clamp returns value limited to the range [low, high]. If low is greater
// than high, the two bounds are swapped first.
func Clamp(value, low, high int) int {
	if low > high {
		low, high = high, low
	}
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
