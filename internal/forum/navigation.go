// Package forum models the forum directory independently of the desktop UI.
package forum

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

type NodeID string
type BoardID string
type NodeKind string
type Order string

const (
	KindForum    NodeKind = "forum"
	KindSubForum NodeKind = "sub_forum"
	KindBoard    NodeKind = "board"
	DFS          Order    = "dfs"
	BFS          Order    = "bfs"
)

var ErrNodeNotFound = errors.New("forum node not found")

// Node is a directory entry. A board is a leaf in the directory; its articles
// are loaded separately. Distinct directory entries may reference one board.
type Node struct {
	ID       NodeID   `json:"id"`
	Kind     NodeKind `json:"kind"`
	Name     string   `json:"name"`
	BoardID  BoardID  `json:"boardId,omitempty"`
	Children []NodeID `json:"children,omitempty"`
}

// Navigation is an immutable snapshot. The number of top-level forums comes
// from the source data, rather than being fixed to the current count of ten.
type Navigation struct {
	roots   []NodeID
	nodes   map[NodeID]Node
	parents map[NodeID]NodeID
}

func NewNavigation(roots []NodeID, entries []Node) (*Navigation, error) {
	n := &Navigation{
		roots: slices.Clone(roots), nodes: make(map[NodeID]Node, len(entries)),
		parents: make(map[NodeID]NodeID, len(entries)),
	}
	for _, entry := range entries {
		if strings.TrimSpace(string(entry.ID)) == "" || strings.TrimSpace(entry.Name) == "" {
			return nil, errors.New("directory entries need a stable ID and a name")
		}
		if _, exists := n.nodes[entry.ID]; exists {
			return nil, fmt.Errorf("duplicate node ID %q", entry.ID)
		}
		switch entry.Kind {
		case KindForum, KindSubForum:
			if entry.BoardID != "" {
				return nil, fmt.Errorf("container %q cannot reference a board", entry.ID)
			}
		case KindBoard:
			if strings.TrimSpace(string(entry.BoardID)) == "" || len(entry.Children) != 0 {
				return nil, fmt.Errorf("board entry %q needs a board ID and cannot have directory children", entry.ID)
			}
		default:
			return nil, fmt.Errorf("invalid node kind %q", entry.Kind)
		}
		n.nodes[entry.ID] = cloneNode(entry)
	}

	// Iterative traversal validates reachability and parent relationships without
	// consuming the call stack for deeply nested sub forums.
	queue := slices.Clone(roots)
	for _, id := range roots {
		entry, exists := n.nodes[id]
		if !exists {
			return nil, fmt.Errorf("root %q: %w", id, ErrNodeNotFound)
		}
		if entry.Kind != KindForum {
			return nil, fmt.Errorf("root %q must be a forum", id)
		}
		if _, exists := n.parents[id]; exists {
			return nil, fmt.Errorf("duplicate root %q", id)
		}
		n.parents[id] = ""
	}
	for cursor := 0; cursor < len(queue); cursor++ {
		entry := n.nodes[queue[cursor]]
		for _, childID := range entry.Children {
			child, exists := n.nodes[childID]
			if !exists {
				return nil, fmt.Errorf("child %q of %q: %w", childID, entry.ID, ErrNodeNotFound)
			}
			if child.Kind == KindForum {
				return nil, fmt.Errorf("forum %q must be a root", childID)
			}
			if _, visited := n.parents[childID]; visited {
				return nil, fmt.Errorf("node %q has repeated parents or a cycle", childID)
			}
			n.parents[childID] = entry.ID
			queue = append(queue, childID)
		}
	}
	if len(n.parents) != len(n.nodes) {
		return nil, errors.New("directory contains unreachable nodes or a disconnected cycle")
	}
	return n, nil
}

func cloneNode(node Node) Node {
	node.Children = slices.Clone(node.Children)
	return node
}

func (n *Navigation) Node(id NodeID) (Node, bool) {
	entry, exists := n.nodes[id]
	return cloneNode(entry), exists
}

// Children returns direct children for a clicked forum/sub forum. An empty ID
// means the site overview; a board has no directory children.
func (n *Navigation) Children(id NodeID) ([]Node, error) {
	ids := n.roots
	if id != "" {
		entry, exists := n.nodes[id]
		if !exists {
			return nil, fmt.Errorf("%q: %w", id, ErrNodeNotFound)
		}
		ids = entry.Children
	}
	result := make([]Node, 0, len(ids))
	for _, childID := range ids {
		result = append(result, cloneNode(n.nodes[childID]))
	}
	return result, nil
}

func (n *Navigation) Breadcrumb(id NodeID) ([]Node, error) {
	if id == "" {
		return []Node{}, nil
	}
	if _, exists := n.nodes[id]; !exists {
		return nil, fmt.Errorf("%q: %w", id, ErrNodeNotFound)
	}
	path := make([]Node, 0)
	for current := id; current != ""; current = n.parents[current] {
		path = append(path, cloneNode(n.nodes[current]))
	}
	slices.Reverse(path)
	return path, nil
}

// Boards resolves a clicked scope for browsing, manual search, and agent tools.
// It controls directory discovery only; network page scheduling is separate.
func (n *Navigation) Boards(scope NodeID, order Order) ([]BoardID, error) {
	if order != DFS && order != BFS {
		return nil, fmt.Errorf("unsupported traversal %q", order)
	}
	pending := slices.Clone(n.roots)
	if scope != "" {
		if _, exists := n.nodes[scope]; !exists {
			return nil, fmt.Errorf("%q: %w", scope, ErrNodeNotFound)
		}
		pending = []NodeID{scope}
	}
	if order == DFS {
		slices.Reverse(pending)
	}
	result := make([]BoardID, 0)
	seenBoards := make(map[BoardID]bool)
	for cursor := 0; len(pending) > 0 && (order == DFS || cursor < len(pending)); {
		var id NodeID
		if order == DFS {
			id = pending[len(pending)-1]
			pending = pending[:len(pending)-1]
		} else {
			id = pending[cursor]
			cursor++
		}
		entry := n.nodes[id]
		if entry.Kind == KindBoard {
			if !seenBoards[entry.BoardID] {
				seenBoards[entry.BoardID] = true
				result = append(result, entry.BoardID)
			}
			continue
		}
		if order == DFS {
			for index := len(entry.Children) - 1; index >= 0; index-- {
				pending = append(pending, entry.Children[index])
			}
		} else {
			pending = append(pending, entry.Children...)
		}
	}
	return result, nil
}
