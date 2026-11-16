package desktop

import "github.com/egoist/mygo/ui"

// Small, code-native outline icons; no font glyphs or bitmap dependencies.
var icons = func() map[string]*ui.SVG {
	paths := map[string]string{
		"home":     `<path d="m3 10 9-7 9 7v10H3zM9 20v-7h6v7"/>`,
		"hot":      `<path d="M13 3c2 5-3 6 0 10 2-1 3-3 3-5 4 4 5 12-3 13C3 22 2 13 7 8c0 4 1 4 2 4 2-3 0-5 4-9Z"/>`,
		"star":     `<path d="m12 3 2.8 5.7 6.2.9-4.5 4.4 1.1 6.2-5.6-3-5.6 3 1.1-6.2L3 9.6l6.2-.9Z"/>`,
		"edit":     `<path d="m15 4 5 5M4 20l5-1L21 7l-5-5L4 14z"/>`,
		"search":   `<circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/>`,
		"message":  `<path d="M21 15a3 3 0 0 1-3 3H9l-6 4V6a3 3 0 0 1 3-3h12a3 3 0 0 1 3 3Z"/><path d="M7 8h10M7 12h7"/>`,
		"user":     `<circle cx="12" cy="7" r="4"/><path d="M4 21v-2a8 8 0 0 1 16 0v2"/>`,
		"settings": `<path d="m9 3-1 3-3 1v4l-2 1 2 1v4l3 1 1 3h6l1-3 3-1v-4l2-1-2-1V7l-3-1-1-3Z"/><circle cx="12" cy="12" r="3"/>`,
		"reply":    `<path d="m9 5-7 7 7 7M3 12h11a7 7 0 0 1 7 7"/>`,
		"like":     `<path d="M7 10 12 3c3 0 2 4 1 6h6a2 2 0 0 1 2 2l-2 8a2 2 0 0 1-2 2H7ZM3 10h4v11H3Z"/>`,
		"down":     `<path d="m7 14 5 7c3 0 2-4 1-6h6a2 2 0 0 0 2-2l-2-8a2 2 0 0 0-2-2H7ZM3 3h4v11H3Z"/>`,
		"link":     `<path d="m10 13 4-4M8 15l-1 1a4 4 0 0 1-5-5l5-5a4 4 0 0 1 6 0m-2 12a4 4 0 0 0 6 0l5-5a4 4 0 0 0-5-5l-1 1"/>`,
		"refresh":  `<path d="M20 8a9 9 0 1 0 1 7M20 3v6h-6"/>`,
		"back":     `<path d="m14 5-7 7 7 7"/>`,
		"next":     `<path d="m10 5 7 7-7 7"/>`,
		"plus":     `<path d="M12 4v16M4 12h16"/>`,
		"board":    `<rect x="3" y="4" width="18" height="16" rx="3"/><path d="M3 10h18M9 10v10"/>`,
		"more":     `<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>`,
		"check":    `<path d="m5 12 4 4L20 5"/>`,
		"close":    `<path d="m6 6 12 12M6 18 18 6"/>`,
		"trash":    `<path d="M3 6h18M9 6V3h6v3M6 6l1 15h10l1-15M10 10v7M14 10v7"/>`,
		"clock":    `<circle cx="12" cy="12" r="9"/><path d="M12 6v6l4 2"/>`,
		"mail":     `<rect x="3" y="5" width="18" height="14" rx="2"/><path d="m3 6 9 7 9-7"/>`,
		"attach":   `<path d="m8 12 7-7a3 3 0 0 1 4 4L9 19a5 5 0 0 1-7-7L13 1m-5 11 5-5"/>`,
		"quote":    `<path d="M4 6h6v7H5l-2 5M14 6h6v7h-5l-2 5"/>`,
		"alert":    `<circle cx="12" cy="12" r="9"/><path d="M12 7v6M12 16v1"/>`,
		"copy":     `<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H3V3h12v2"/>`,
		"pin":      `<path d="m12 3 2 6 6 2-5 5 1 5-4-3-4 3 1-5-5-5 6-2Z"/>`,
		"poll":     `<path d="M4 20V11M10 20V4M16 20v-6M22 20H2"/>`,
		"bet":      `<path d="M8 21h8M12 17v4M7 4h10v4a5 5 0 0 1-10 0ZM7 6H4a2 2 0 0 0 2 3h1M17 6h3a2 2 0 0 1-2 3h-1"/>`,
		"send":     `<path d="m22 2-7 20-4-9-9-4Z"/><path d="M22 2 11 13"/>`,
	}
	out := make(map[string]*ui.SVG, len(paths))
	for name, path := range paths {
		out[name] = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">` + path + `</svg>`))
	}
	return out
}()
