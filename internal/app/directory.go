package app

import "BYR-Client/internal/forum"

// DirectoryCounts returns the boards and sub forums below a directory node.
// A board entry that aliases the same board is counted once, matching the
// navigation scope resolver.
func (a *App) DirectoryCounts(node forum.NodeID) (boards, subForums int) {
	nav := a.Navigation()
	if nav == nil {
		return 0, 0
	}
	var start []forum.NodeID
	if node == "" {
		children, err := nav.Children("")
		if err != nil {
			return 0, 0
		}
		for _, child := range children {
			start = append(start, child.ID)
		}
	} else {
		entry, ok := nav.Node(node)
		if !ok {
			return 0, 0
		}
		start = append(start, entry.Children...)
	}
	seenBoards := make(map[forum.BoardID]bool)
	pending := append([]forum.NodeID(nil), start...)
	for len(pending) > 0 {
		id := pending[0]
		pending = pending[1:]
		entry, ok := nav.Node(id)
		if !ok {
			continue
		}
		switch entry.Kind {
		case forum.KindBoard:
			if !seenBoards[entry.BoardID] {
				seenBoards[entry.BoardID] = true
				boards++
			}
			continue
		case forum.KindSubForum:
			subForums++
		}
		pending = append(pending, entry.Children...)
	}
	return boards, subForums
}
