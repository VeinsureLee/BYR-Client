package forum

import (
	"errors"
	"reflect"
	"testing"
)

func exampleNavigation(t *testing.T) *Navigation {
	t.Helper()
	n, err := NewNavigation([]NodeID{"f1", "f2"}, []Node{
		{ID: "f1", Name: "Forum 1", Kind: KindForum, Children: []NodeID{"s1", "near"}},
		{ID: "s1", Name: "Sub forum", Kind: KindSubForum, Children: []NodeID{"s2"}},
		{ID: "s2", Name: "Nested sub forum", Kind: KindSubForum, Children: []NodeID{"deep"}},
		{ID: "deep", Name: "Deep board", Kind: KindBoard, BoardID: "A"},
		{ID: "near", Name: "Direct board", Kind: KindBoard, BoardID: "B"},
		{ID: "f2", Name: "Forum 2", Kind: KindForum, Children: []NodeID{"other", "alias"}},
		{ID: "other", Name: "Other board", Kind: KindBoard, BoardID: "C"},
		{ID: "alias", Name: "Alias of deep board", Kind: KindBoard, BoardID: "A"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestClickedScopeAndTraversal(t *testing.T) {
	n := exampleNavigation(t)
	for _, test := range []struct {
		name  string
		scope NodeID
		order Order
		want  []BoardID
	}{
		{"all depth first", "", DFS, []BoardID{"A", "B", "C"}},
		{"all breadth first", "", BFS, []BoardID{"B", "C", "A"}},
		{"forum", "f1", DFS, []BoardID{"A", "B"}},
		{"sub forum", "s1", BFS, []BoardID{"A"}},
		{"board", "deep", BFS, []BoardID{"A"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := n.Boards(test.scope, test.order)
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("Boards = %v, %v; want %v", got, err, test.want)
			}
		})
	}
	path, err := n.Breadcrumb("deep")
	if err != nil {
		t.Fatal(err)
	}
	var ids []NodeID
	for _, entry := range path {
		ids = append(ids, entry.ID)
	}
	if !reflect.DeepEqual(ids, []NodeID{"f1", "s1", "s2", "deep"}) {
		t.Fatalf("incorrect path: %v", ids)
	}
	children, err := n.Children("f1")
	if err != nil || len(children) != 2 || children[0].ID != "s1" {
		t.Fatalf("incorrect clicked overview: %v, %v", children, err)
	}
	children, err = n.Children("deep")
	if err != nil || len(children) != 0 {
		t.Fatalf("board articles must not be directory children: %v, %v", children, err)
	}
}

func TestNavigationSnapshotCannotBeMutatedByCaller(t *testing.T) {
	roots := []NodeID{"f"}
	entries := []Node{
		{ID: "f", Kind: KindForum, Name: "Forum", Children: []NodeID{"b"}},
		{ID: "b", Kind: KindBoard, Name: "Board", BoardID: "B"},
	}
	n, err := NewNavigation(roots, entries)
	if err != nil {
		t.Fatal(err)
	}
	roots[0] = "bad"
	entries[0].Children[0] = "bad"
	entry, _ := n.Node("f")
	entry.Children[0] = "bad"
	path, _ := n.Breadcrumb("b")
	path[0].Children[0] = "bad"
	children, _ := n.Children("")
	children[0].Children[0] = "bad"
	got, err := n.Boards("", BFS)
	if err != nil || !reflect.DeepEqual(got, []BoardID{"B"}) {
		t.Fatalf("snapshot was mutated: %v, %v", got, err)
	}
}

func TestRejectInvalidDirectories(t *testing.T) {
	for _, test := range []struct {
		name    string
		roots   []NodeID
		entries []Node
	}{
		{"missing root", []NodeID{"missing"}, nil},
		{"non-forum root", []NodeID{"s"}, []Node{{ID: "s", Name: "Sub", Kind: KindSubForum}}},
		{"duplicate roots", []NodeID{"f", "f"}, []Node{{ID: "f", Name: "Forum", Kind: KindForum}}},
		{"missing child", []NodeID{"f"}, []Node{{ID: "f", Name: "Forum", Kind: KindForum, Children: []NodeID{"missing"}}}},
		{"unreachable node", nil, []Node{{ID: "f", Name: "Forum", Kind: KindForum}}},
		{"duplicate ID", nil, []Node{{ID: "f", Name: "A", Kind: KindForum}, {ID: "f", Name: "B", Kind: KindForum}}},
		{"board with children", nil, []Node{{ID: "b", Name: "Board", Kind: KindBoard, BoardID: "B", Children: []NodeID{"s"}}}},
		{"cycle", []NodeID{"f"}, []Node{
			{ID: "f", Name: "Forum", Kind: KindForum, Children: []NodeID{"s"}},
			{ID: "s", Name: "Sub", Kind: KindSubForum, Children: []NodeID{"s"}},
		}},
		{"repeated parent", []NodeID{"f"}, []Node{
			{ID: "f", Name: "Forum", Kind: KindForum, Children: []NodeID{"s", "b"}},
			{ID: "s", Name: "Sub", Kind: KindSubForum, Children: []NodeID{"b"}},
			{ID: "b", Name: "Board", Kind: KindBoard, BoardID: "B"},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewNavigation(test.roots, test.entries); err == nil {
				t.Fatal("invalid directory was accepted")
			}
		})
	}
}

func TestUnknownScopeDoesNotExpandToWholeSite(t *testing.T) {
	n := exampleNavigation(t)
	if _, err := n.Boards("missing", BFS); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("expected missing scope error, got %v", err)
	}
	if _, err := n.Boards("f1", "invalid"); err == nil {
		t.Fatal("invalid traversal was accepted")
	}
}
