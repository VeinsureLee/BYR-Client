package content

// Editor selections use rune offsets. Keeping this here prevents each UI
// framework from inventing its own UTF-8/UTF-16 slicing rules for UBB actions.
func selection(text string, start, end int) ([]rune, int, int) {
	r := []rune(text)
	start = max(0, min(start, len(r)))
	end = max(0, min(end, len(r)))
	if start > end {
		start, end = end, start
	}
	return r, start, end
}
func WrapSelection(text string, start, end int, tag string) (string, int, int) {
	r, start, end := selection(text, start, end)
	before := "[" + tag + "]"
	after := "[/" + tag + "]"
	next := string(r[:start]) + before + string(r[start:end]) + after + string(r[end:])
	return next, start + len([]rune(before)), end + len([]rune(before))
}
func Insert(text string, start, end int, value string) (string, int) {
	r, start, end := selection(text, start, end)
	next := string(r[:start]) + value + string(r[end:])
	return next, start + len([]rune(value))
}
