// Package memory provides the initial in-process observation store. It keeps
// the same write rules as the planned SQLite adapter: entities are matched by
// stable identity and missing fields never erase stored content.
package memory

import (
	"slices"
	"sync"

	"BYR-Client/internal/domain"
	"BYR-Client/internal/forum"
	"BYR-Client/internal/ports"
)

type Store struct {
	mu      sync.RWMutex
	catalog domain.Catalog
	hasCat  bool

	articles map[domain.ArticleKey]domain.Article
	replies  map[domain.ArticleKey][]domain.Reply
	stats    ports.StoreStats
}

func New() *Store {
	return &Store{
		articles: make(map[domain.ArticleKey]domain.Article),
		replies:  make(map[domain.ArticleKey][]domain.Reply),
	}
}

func (s *Store) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.catalog = domain.Catalog{}
	s.hasCat = false
	s.articles = make(map[domain.ArticleKey]domain.Article)
	s.replies = make(map[domain.ArticleKey][]domain.Reply)
	s.stats = ports.StoreStats{}
}

func (s *Store) SaveCatalog(c domain.Catalog) error {
	c.Roots = slices.Clone(c.Roots)
	nodes := c.Nodes
	c.Nodes = make([]forum.Node, len(nodes))
	for i, node := range nodes {
		node.Children = slices.Clone(node.Children)
		c.Nodes[i] = node
	}
	s.mu.Lock()
	s.catalog = c
	s.hasCat = true
	s.mu.Unlock()
	return nil
}
func (s *Store) Catalog() (domain.Catalog, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.hasCat {
		return domain.Catalog{}, false
	}
	c := s.catalog
	c.Roots = slices.Clone(c.Roots)
	nodes := c.Nodes
	c.Nodes = make([]forum.Node, len(nodes))
	for i, node := range nodes {
		node.Children = slices.Clone(node.Children)
		c.Nodes[i] = node
	}
	return c, true
}

func (s *Store) UpsertArticle(a domain.Article) (bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, exists := s.articles[a.Key]
	if !exists {
		s.articles[a.Key] = a
		s.stats.Articles++
		s.stats.InsertedArticles++
		return true, false
	}
	// A list summary must not erase a body or images loaded earlier.
	if a.Content == "" {
		if old.Content != "" {
			a.Content = old.Content
		}
		a.BodyLoaded = old.BodyLoaded
		if len(a.Images) == 0 {
			a.Images = slices.Clone(old.Images)
		}
	}
	if sameArticle(old, a) {
		return false, false
	}
	s.articles[a.Key] = a
	s.stats.UpdatedArticles++
	return false, true
}

func (s *Store) UpsertReply(r domain.Reply) (bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.replies[r.Key.ArticleKey]
	for i := range list {
		if list[i].Key.ReplyID != r.Key.ReplyID {
			continue
		}
		if sameReply(list[i], r) {
			return false, false
		}
		list[i] = r
		s.replies[r.Key.ArticleKey] = list
		return false, true
	}
	s.replies[r.Key.ArticleKey] = append(list, r)
	s.stats.Replies++
	s.stats.InsertedReplies++
	return true, false
}

func (s *Store) Article(key domain.ArticleKey) (domain.Article, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.articles[key]
	return a, ok
}

func (s *Store) Replies(key domain.ArticleKey) ([]domain.Reply, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list, ok := s.replies[key]
	if !ok {
		return nil, false
	}
	return slices.Clone(list), true
}

func (s *Store) Stats() ports.StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

func sameArticle(a, b domain.Article) bool {
	return a.Title == b.Title &&
		a.Author == b.Author &&
		a.Pinned == b.Pinned &&
		a.ReplyCount == b.ReplyCount &&
		a.CreatedAt.Equal(b.CreatedAt) &&
		a.LastReplyAt.Equal(b.LastReplyAt) &&
		a.Summary == b.Summary &&
		a.Content == b.Content &&
		slices.Equal(a.Images, b.Images) &&
		a.BodyLoaded == b.BodyLoaded
}

func sameReply(a, b domain.Reply) bool {
	return a.Floor == b.Floor &&
		a.Author == b.Author &&
		a.Content == b.Content &&
		slices.Equal(a.Images, b.Images) &&
		a.QuoteAuthor == b.QuoteAuthor &&
		a.QuoteContent == b.QuoteContent &&
		a.CreatedAt.Equal(b.CreatedAt)
}
