package application

import (
	"context"
	"io"

	"github.com/VeinsureLee/BYR-Client/core/forum"
)

// TokenSource is a non-retainable-by-the-UI token capability supplied only to
// the private protocol adapter during core assembly.
type TokenSource func(context.Context) (string, error)

// ForumClient contains only the protocol operations consumed by application
// use cases. Concrete HTTP implementation lives in the core-private adapter.
type ForumClient interface {
	Login(context.Context, string, string) (forum.OAuthInfo, error)
	CheckWebLogin(context.Context, string, string) (forum.WebLogin, error)

	Sections(context.Context) (forum.SectionList, error)
	Section(context.Context, string) (forum.Section, error)
	Board(context.Context, string, forum.Page) (forum.Board, error)
	Thread(context.Context, string, string, forum.Page) (forum.Thread, error)
	Article(context.Context, string, string) (forum.Article, error)
	PostArticle(context.Context, forum.Post) (forum.Article, error)
	UpdateArticle(context.Context, string, string, string, string) (forum.Article, error)
	DeleteArticle(context.Context, string, string) (forum.Article, error)

	Like(context.Context, string, string) (forum.Reaction, error)
	VoteDown(context.Context, string, string) (forum.Reaction, error)
	Attachments(context.Context, string, string) (forum.Attachment, error)
	Upload(context.Context, string, string, string, io.Reader) (forum.Attachment, error)
	DeleteAttachment(context.Context, string, string) (forum.Attachment, error)
	FetchAsset(context.Context, string) ([]byte, error)
	ThreadURL(string, string) string

	SearchArticles(context.Context, forum.Search) (forum.SearchResults, error)
	SearchBoards(context.Context, string) (forum.BoardList, error)
	TopTen(context.Context) (forum.Widget, error)
	Recommendations(context.Context) (forum.Recommendations, error)
	FavorPosts(context.Context, forum.Page) (forum.ArticleList, error)

	Self(context.Context) (forum.User, error)
	User(context.Context, string) (forum.User, error)
	Blacklist(context.Context, int) (forum.Blacklist, error)
	AddBlacklist(context.Context, string) (forum.Status, error)
	RemoveBlacklist(context.Context, string) (forum.Status, error)
	Favorites(context.Context, string) (forum.Favorite, error)
	AddFavorite(context.Context, string, string) (forum.Favorite, error)
	RemoveFavorite(context.Context, string, string) (forum.Favorite, error)
	Collection(context.Context, string, forum.Page) (forum.Collection, error)
	AddCollection(context.Context, string, string) (forum.Article, error)
	RemoveCollection(context.Context, string, string) (forum.Article, error)

	Mailbox(context.Context, string, forum.Page) (forum.Mailbox, error)
	Mail(context.Context, string, string) (forum.Mail, error)
	SendMail(context.Context, string, string, string) (forum.Status, error)
	ReplyMail(context.Context, string, string, string, string) (forum.Mail, error)
	ForwardMail(context.Context, string, string, string) (forum.Mail, error)
	DeleteMail(context.Context, string, string) (forum.Mail, error)
	Refers(context.Context, string, forum.Page) (forum.Refers, error)
	ReadRefer(context.Context, string, string) (forum.Refers, error)
	ReadAllRefers(context.Context, string) (forum.Status, error)

	Vote(context.Context, string) (forum.Vote, error)
	SubmitVote(context.Context, string, string) (forum.Vote, error)
	Bet(context.Context, string) (forum.Bet, error)
	SubmitBet(context.Context, string, string, int) (forum.Bet, error)
	Version(context.Context, bool) (forum.Versions, error)
}

// ForumFactory lets the composition root build a private protocol adapter after
// the application facade exists, so its token callback is bound to that facade.
type ForumFactory func(TokenSource, func() bool) (ForumClient, error)
