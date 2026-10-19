// Package ports defines the boundaries between application services, forum
// data sources and local storage.
package ports

import (
	"context"
	"errors"
	"time"

	"BYR-Client/internal/domain"
)

var ErrAuthRequired = errors.New("authentication required")

type Session struct {
	SiteID      string
	AccountID   string
	DisplayName string
	Epoch       uint64
	LoggedIn    bool
}

type Authenticator interface {
	Login(context.Context, string, string) (Session, error)
	Restore(context.Context) (Session, bool, error)
	Logout(context.Context) error
	Session() Session
}

type ImageReader interface {
	ReadImage(context.Context, string) ([]byte, error)
}

type TrafficSource interface {
	ReadSectionTraffic(context.Context, domain.Scope, string, []domain.BoardID) (map[domain.BoardID]domain.Traffic, error)
}

type Cookie struct {
	Name     string
	Value    string
	Domain   string
	Path     string
	Secure   bool
	HTTPOnly bool
}

type CookieProvider interface {
	Cookies() []Cookie
}

type URLProvider interface {
	ArticleURL(domain.ArticleKey) string
}

type ForumSource interface {
	ReadCatalog(context.Context) (domain.Catalog, error)
	ReadBoardPage(context.Context, domain.Scope, domain.BoardID, domain.Cursor, int) (domain.BoardPage, error)
	ReadThreadPage(context.Context, domain.Scope, domain.ArticleKey, domain.Cursor, int) (domain.ThreadPage, error)
	SearchBoardPage(context.Context, domain.Scope, domain.BoardID, domain.BoardQuery, domain.Cursor, int) (domain.BoardPage, error)
}

type StoreStats struct {
	Articles         int
	Replies          int
	InsertedArticles int
	UpdatedArticles  int
	InsertedReplies  int
}

type ObservationStore interface {
	SaveCatalog(domain.Catalog) error
	Catalog() (domain.Catalog, bool)
	UpsertArticle(domain.Article) (created, updated bool)
	UpsertReply(domain.Reply) (created, updated bool)
	Article(domain.ArticleKey) (domain.Article, bool)
	Replies(domain.ArticleKey) ([]domain.Reply, bool)
	Stats() StoreStats
}

type Clock interface {
	Now() time.Time
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }
