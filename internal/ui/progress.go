package ui

const (
	discoveryBarWidth = 24
	sizingBarWidth    = 24
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func indeterminateBar(frame, width int) string {
	if width <= 0 {
		return "[]"
	}
	cells := make([]byte, width)
	for i := range cells {
		cells[i] = ' '
	}
	pos := frame % width
	cells[pos] = '='
	if pos > 0 {
		cells[pos-1] = '-'
	}
	if pos+1 < width {
		cells[pos+1] = '-'
	}
	return "[" + string(cells) + "]"
}

func ratioBar(done, total, width int) string {
	if width <= 0 {
		return "[]"
	}
	if total <= 0 {
		return "[" + string(make([]byte, width)) + "]"
	}
	clamped := done
	if clamped < 0 {
		clamped = 0
	}
	if clamped > total {
		clamped = total
	}
	filled := (clamped * width) / total
	bar := make([]byte, width)
	for i := 0; i < filled; i++ {
		bar[i] = '#'
	}
	for i := filled; i < width; i++ {
		bar[i] = ' '
	}
	return "[" + string(bar) + "]"
}
