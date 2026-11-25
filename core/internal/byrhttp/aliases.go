package byrhttp

import "github.com/VeinsureLee/BYR-Client/core/forum"

// Aliases keep protocol request code typed with the stable shared forum values
// while allowing the transport implementation to live behind core/internal.
type (
	Scalar          = forum.Scalar
	Number          = forum.Number
	Flag            = forum.Flag
	Page            = forum.Page
	SectionList     = forum.SectionList
	Section         = forum.Section
	Board           = forum.Board
	BoardList       = forum.BoardList
	Thread          = forum.Thread
	Article         = forum.Article
	ArticleList     = forum.ArticleList
	Post            = forum.Post
	Reaction        = forum.Reaction
	Attachment      = forum.Attachment
	Favorite        = forum.Favorite
	Collection      = forum.Collection
	Widget          = forum.Widget
	Recommendations = forum.Recommendations
	Search          = forum.Search
	SearchResults   = forum.SearchResults
	User            = forum.User
	Blacklist       = forum.Blacklist
	Mailbox         = forum.Mailbox
	Mail            = forum.Mail
	Status          = forum.Status
	Refers          = forum.Refers
	ReferInfo       = forum.ReferInfo
	Vote            = forum.Vote
	Bet             = forum.Bet
	Banners         = forum.Banners
	Welcome         = forum.Welcome
	Versions        = forum.Versions
	OAuthInfo       = forum.OAuthInfo
	WebLogin        = forum.WebLogin
	APIError        = forum.APIError
	HTTPError       = forum.HTTPError
	DecodeError     = forum.DecodeError
)

var (
	ErrUnauthenticated  = forum.ErrUnauthenticated
	ErrEmptyResponse    = forum.ErrEmptyResponse
	ErrResponseTooLarge = forum.ErrResponseTooLarge
	ErrRedirectOrigin   = forum.ErrRedirectOrigin
)
