package ui

// pageJumpIndex moves selection by delta rows (one page when delta is ±pageSize).
func pageJumpIndex(selected, total, delta int) int {
	if total <= 0 {
		return 0
	}
	next := selected + delta
	if next < 0 {
		return 0
	}
	if next >= total {
		return total - 1
	}
	return next
}
