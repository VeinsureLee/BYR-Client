// Package domain contains the forum data model shared by the application
// services, sources and stores. It does not depend on the desktop UI.
package domain

import (
	"time"

	"BYR-Client/internal/forum"
)

const (
	DefaultBoardBatchSize   = 20
	DefaultPerBoardLimit    = 20
	DefaultTotalLimit       = 200
	DefaultThreadPages      = 5
	DefaultConcurrency      = 5
	DefaultRepliesPerPage   = 10
	DefaultSearchCandidates = 20
)

type Scope struct {
	SiteID       string
	AccountID    string
	SessionEpoch uint64
}

type BoardID = forum.BoardID

type ArticleID string
type ReplyID string
type Cursor string

type ArticleKey struct {
	SiteID    string
	BoardID   BoardID
	ArticleID ArticleID
}

type ReplyKey struct {
	ArticleKey
	ReplyID ReplyID
}

// Article is a topic. Its content may be a summary from a board list or the
// loaded body from a thread read; BodyLoaded distinguishes the two.
type ImageKind string

const (
	ImagePhoto   ImageKind = "photo"
	ImageEmoji   ImageKind = "emoji"
	ImageUnknown ImageKind = "unknown"
)

type Image struct {
	URL    string    `json:"url"`
	Kind   ImageKind `json:"kind,omitempty"`
	Width  int       `json:"width,omitempty"`
	Height int       `json:"height,omitempty"`
}

type Article struct {
	Key         ArticleKey `json:"key"`
	Title       string     `json:"title"`
	Author      string     `json:"author"`
	Pinned      bool       `json:"pinned"`
	ReplyCount  int        `json:"replyCount"`
	CreatedAt   time.Time  `json:"createdAt"`
	LastReplyAt time.Time  `json:"lastReplyAt"`
	Summary     string     `json:"summary,omitempty"`
	Content     string     `json:"content,omitempty"`
	Images      []Image    `json:"images,omitempty"`
	BodyLoaded  bool       `json:"bodyLoaded"`
}

type Reply struct {
	Key          ReplyKey  `json:"key"`
	Floor        int       `json:"floor"`
	Author       string    `json:"author"`
	Content      string    `json:"content"`
	Images       []Image   `json:"images,omitempty"`
	QuoteAuthor  string    `json:"quoteAuthor,omitempty"`
	QuoteContent string    `json:"quoteContent,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Catalog struct {
	Roots     []forum.NodeID `json:"roots"`
	Nodes     []forum.Node   `json:"nodes"`
	FetchedAt time.Time      `json:"fetchedAt"`
	Warnings  []string       `json:"warnings,omitempty"`
}

type Traffic struct {
	Known       bool      `json:"known"`
	OnlineUsers int       `json:"onlineUsers"`
	TodayPosts  int       `json:"todayPosts"`
	Threads     int       `json:"threads"`
	Posts       int       `json:"posts"`
	ObservedAt  time.Time `json:"observedAt"`
}

func (t Traffic) Add(other Traffic) Traffic {
	if !t.Known {
		return other
	}
	if !other.Known {
		return t
	}
	return Traffic{
		Known:       true,
		OnlineUsers: t.OnlineUsers + other.OnlineUsers,
		TodayPosts:  t.TodayPosts + other.TodayPosts,
		Threads:     t.Threads + other.Threads,
		Posts:       t.Posts + other.Posts,
		ObservedAt:  laterTime(t.ObservedAt, other.ObservedAt),
	}
}

func laterTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

type BoardPage struct {
	BoardID    BoardID   `json:"boardId"`
	Articles   []Article `json:"articles"`
	Next       Cursor    `json:"next,omitempty"`
	End        bool      `json:"end"`
	ObservedAt time.Time `json:"observedAt"`
}

type ThreadPage struct {
	Article    Article   `json:"article"`
	Replies    []Reply   `json:"replies"`
	Next       Cursor    `json:"next,omitempty"`
	End        bool      `json:"end"`
	PagesRead  int       `json:"pagesRead"`
	ObservedAt time.Time `json:"observedAt"`
}

type TimeRangeKind string

const (
	TimeAll    TimeRangeKind = "all"
	TimeWeek   TimeRangeKind = "week"
	TimeMonth  TimeRangeKind = "month"
	TimeCustom TimeRangeKind = "custom"
)

type TimeRange struct {
	Kind TimeRangeKind `json:"kind"`
	From time.Time     `json:"from,omitempty"`
	To   time.Time     `json:"to,omitempty"`
}

// Contains reports whether an article time is inside the requested range.
func (r TimeRange) Contains(t time.Time) bool {
	if !r.From.IsZero() && t.Before(r.From) {
		return false
	}
	if !r.To.IsZero() && t.After(r.To) {
		return false
	}
	return true
}

type SearchQuery struct {
	Keywords    []string     `json:"keywords,omitempty"`
	Author      string       `json:"author,omitempty"`
	Time        TimeRange    `json:"time"`
	ScopeNode   forum.NodeID `json:"scopeNode,omitempty"`
	PerBoard    int          `json:"perBoard"`
	Total       int          `json:"total"`
	ThreadPages int          `json:"threadPages"`
	Concurrency int          `json:"concurrency"`
}

func (q SearchQuery) Normalized() SearchQuery {
	if q.PerBoard <= 0 {
		q.PerBoard = DefaultPerBoardLimit
	}
	if q.Total <= 0 {
		q.Total = DefaultTotalLimit
	}
	if q.ThreadPages <= 0 {
		q.ThreadPages = DefaultThreadPages
	}
	if q.Concurrency <= 0 {
		q.Concurrency = DefaultConcurrency
	}
	return q
}

func (q SearchQuery) Empty() bool {
	return len(q.Keywords) == 0 && q.Author == ""
}

// BoardQuery is the candidate request handed to a source. Filtering for the
// final result set stays in the application service.
type BoardQuery struct {
	Keywords []string
	Author   string
	From     time.Time
	To       time.Time
}
