// Protocol DTOs migrated from the archived Flutter JSON contracts.
package forum

import "encoding/json"

type Article struct {
	ID                Scalar           `json:"id"`
	GroupID           Scalar           `json:"group_id"`
	ReplyID           Scalar           `json:"reply_id"`
	Flag              string           `json:"flag"`
	Position          Number           `json:"position"`
	IsTop             Flag             `json:"is_top"`
	IsSubject         Flag             `json:"is_subject"`
	HasAttachment     Flag             `json:"has_attachment"`
	IsAdmin           Flag             `json:"is_admin"`
	Title             string           `json:"title"`
	Author            User             `json:"user"`
	PostTime          Scalar           `json:"post_time"`
	Board             string           `json:"board_name"`
	BoardDescription  string           `json:"board_description"`
	ReplyCount        Number           `json:"reply_count"`
	LastReplyUserID   Scalar           `json:"last_reply_user_id"`
	LastReplyTime     Scalar           `json:"last_reply_time"`
	Content           string           `json:"content"`
	Attachments       List[Attachment] `json:"attachment"`
	PreviousID        Scalar           `json:"previous_id"`
	NextID            Scalar           `json:"next_id"`
	ThreadsPreviousID Scalar           `json:"threads_previous_id"`
	ThreadsNextID     Scalar           `json:"threads_next_id"`
	LikeSum           Number           `json:"like_sum"`
	IsLiked           Flag             `json:"is_liked"`
	VotedownSum       Number           `json:"votedown_sum"`
	IsVotedown        Flag             `json:"is_votedown"`
	HiddenByVotedown  Flag             `json:"hidden_by_votedown"`
	IDCount           Number           `json:"id_count"`
}

type File struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	Size            Scalar `json:"size"`
	ThumbnailSmall  string `json:"thumbnail_small"`
	ThumbnailMiddle string `json:"thumbnail_middle"`
}

type Attachment struct {
	Files       List[File] `json:"file"`
	RemainSpace Scalar     `json:"remain_space"`
	RemainCount Number     `json:"remain_count"`
}

type Banner struct {
	ImageURL string `json:"image_url"`
	Intro    string `json:"intro"`
	URL      string `json:"url"`
}

type Banners struct {
	Count   Number       `json:"count"`
	Banners List[Banner] `json:"banners"`
}

type MyBet struct {
	UserID Scalar `json:"uid"`
	ItemID Scalar `json:"biid"`
	Bonus  Number `json:"bonus"`
}

type BetItem struct {
	ID      Scalar `json:"biid"`
	Label   string `json:"label"`
	Percent Scalar `json:"percent"`
	Num     Number `json:"num"`
}

type Bet struct {
	ID          Scalar        `json:"bid"`
	Cate        Scalar        `json:"cate"`
	Title       string        `json:"title"`
	Description string        `json:"desc"`
	Start       Scalar        `json:"start"`
	End         Scalar        `json:"end"`
	Num         Number        `json:"num"`
	Limit       Number        `json:"limit"`
	Discount    Number        `json:"discount"`
	Score       Number        `json:"score"`
	ArticleID   Scalar        `json:"aid"`
	Result      Scalar        `json:"result"`
	Items       List[BetItem] `json:"bitems"`
	MyBet       *MyBet        `json:"mybet"`
}

type Blacklist struct {
	Users      List[User] `json:"user"`
	Pagination Pagination `json:"pagination"`
}

type Board struct {
	Name               string          `json:"name"`
	Managers           List[User]      `json:"manager"`
	Description        string          `json:"description"`
	BoardClass         string          `json:"class"`
	Section            json.RawMessage `json:"section"`
	IsFavorite         Flag            `json:"is_favorite"`
	ThreadsTodayCount  Number          `json:"threads_today_count"`
	AllowAttachment    Flag            `json:"allow_attachment"`
	PostTodayCount     Number          `json:"post_today_count"`
	PostThreadsCount   Number          `json:"post_threads_count"`
	PostAllCount       Number          `json:"post_all_count"`
	UserOnlineCount    Number          `json:"user_online_count"`
	IsReadOnly         Flag            `json:"is_read_only"`
	IsNoReply          Flag            `json:"is_no_reply"`
	AllowAnonymous     Flag            `json:"allow_anonymous"`
	AllowOutgo         Flag            `json:"allow_outgo"`
	AllowPost          *Flag           `json:"allow_post"`
	UserOnlineMaxCount Number          `json:"user_online_max_count"`
	UserOnlineMaxTime  Scalar          `json:"user_online_max_time"`
	Pagination         Pagination      `json:"pagination"`
	Articles           List[Article]   `json:"article"`
}

type CollectionEntry struct {
	CreatedTime Scalar `json:"createdTime"`
	Title       string `json:"title"`
	PostTime    Scalar `json:"postTime"`
	Board       string `json:"bname"`
	ReplyCount  Number `json:"num"`
	ID          Scalar `json:"gid"`
	Author      User   `json:"user"`
}

type Collection struct {
	Entries    List[CollectionEntry] `json:"article"`
	Pagination Pagination            `json:"pagination"`
}

type ArticleList struct {
	Articles   List[Article] `json:"article"`
	Pagination Pagination    `json:"pagination"`
}

type Favorite struct {
	SubFavorites List[Favorite] `json:"sub_favorite"`
	Sections     List[Section]  `json:"section"`
	Boards       List[Board]    `json:"board"`
}

type Reaction struct {
	Status      Scalar  `json:"status"`
	LikeSum     *Number `json:"like_sum"`
	VotedownSum *Number `json:"votedown_sum"`
}

type Mail struct {
	Index         Scalar           `json:"index"`
	IsM           Flag             `json:"is_m"`
	IsRead        Flag             `json:"is_read"`
	IsReply       Flag             `json:"is_reply"`
	HasAttachment Flag             `json:"has_attachment"`
	Title         string           `json:"title"`
	Author        User             `json:"user"`
	PostTime      Scalar           `json:"post_time"`
	Box           string           `json:"box_name"`
	Content       string           `json:"content"`
	Attachments   List[Attachment] `json:"attachment"`
}

type Mailbox struct {
	Description string     `json:"description"`
	NewNum      Number     `json:"new_num"`
	Pagination  Pagination `json:"pagination"`
	Mails       List[Mail] `json:"mail"`
}

type OAuthInfo struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   Number `json:"expires_in"`
}

type Recommendations struct {
	Name     string        `json:"name"`
	Title    string        `json:"title"`
	Time     Scalar        `json:"time"`
	Articles List[Article] `json:"article"`
}

type ReferEntry struct {
	Index   Scalar `json:"index"`
	ID      Scalar `json:"id"`
	GroupID Scalar `json:"group_id"`
	ReplyID Scalar `json:"reply_id"`
	Board   string `json:"board_name"`
	Pos     Number `json:"pos"`
	Author  User   `json:"user"`
	Title   string `json:"title"`
	Time    Scalar `json:"time"`
	IsRead  Flag   `json:"is_read"`
}

type ReferInfo struct {
	Description string `json:"description"`
	Enable      Flag   `json:"enable"`
	NewCount    Number `json:"new_count"`
}

type Refers struct {
	Pagination Pagination       `json:"pagination"`
	Articles   List[ReferEntry] `json:"article"`
}

type Pagination struct {
	TotalPages   Number `json:"page_all_count"`
	CurrentPage  Number `json:"page_current_count"`
	ItemsPerPage Number `json:"item_page_count"`
	TotalItems   Number `json:"item_all_count"`
}

type SectionList struct {
	SectionCount Number        `json:"section_count"`
	Sections     List[Section] `json:"section"`
}

type SearchResults struct {
	Pagination Pagination    `json:"pagination"`
	Articles   List[Article] `json:"article"`
}

type BoardList struct {
	Sections List[Section] `json:"section"`
	Boards   List[Board]   `json:"board"`
}

type Section struct {
	Name        Scalar        `json:"name"`
	Description string        `json:"description"`
	IsRoot      Flag          `json:"is_root"`
	Parent      Scalar        `json:"parent"`
	Subsections List[Section] `json:"sub_section"`
	Boards      List[Board]   `json:"board"`
}

type Status struct {
	Status Scalar `json:"status"`
}

type Thread struct {
	ID               Scalar        `json:"id"`
	GroupID          Scalar        `json:"group_id"`
	ReplyID          Scalar        `json:"reply_id"`
	Flag             string        `json:"flag"`
	Position         Number        `json:"position"`
	IsTop            Flag          `json:"is_top"`
	IsSubject        Flag          `json:"is_subject"`
	HasAttachment    Flag          `json:"has_attachment"`
	IsAdmin          Flag          `json:"is_admin"`
	Title            string        `json:"title"`
	Author           User          `json:"user"`
	PostTime         Scalar        `json:"post_time"`
	Board            string        `json:"board_name"`
	BoardDescription string        `json:"board_description"`
	LikeSum          Number        `json:"like_sum"`
	IsLiked          Flag          `json:"is_liked"`
	VotedownSum      Number        `json:"votedown_sum"`
	IsVotedown       Flag          `json:"is_votedown"`
	HiddenByVotedown Flag          `json:"hidden_by_votedown"`
	Collect          Flag          `json:"collect"`
	ReplyCount       Number        `json:"reply_count"`
	LastReplyUserID  Scalar        `json:"last_reply_user_id"`
	LastReplyTime    Scalar        `json:"last_reply_time"`
	LikeArticles     List[Article] `json:"like_articles"`
	Pagination       Pagination    `json:"pagination"`
	Articles         List[Article] `json:"article"`
}

type Widget struct {
	Name     string        `json:"name"`
	Title    string        `json:"title"`
	Time     Scalar        `json:"time"`
	Articles List[Article] `json:"article"`
}

type User struct {
	ID            Scalar `json:"id"`
	UserName      string `json:"user_name"`
	FaceURL       string `json:"face_url"`
	FaceWidth     Number `json:"face_width"`
	FaceHeight    Number `json:"face_height"`
	Gender        string `json:"gender"`
	Astro         string `json:"astro"`
	Life          Scalar `json:"life"`
	QQ            Scalar `json:"qq"`
	MSN           string `json:"msn"`
	HomePage      string `json:"home_page"`
	Level         Scalar `json:"level"`
	IsOnline      Flag   `json:"is_online"`
	PostCount     Number `json:"post_count"`
	LastLoginTime Scalar `json:"last_login_time"`
	LastLoginIP   string `json:"last_login_ip"`
	IsHide        Flag   `json:"is_hide"`
	IsRegister    Flag   `json:"is_register"`
	Score         Number `json:"score"`
	FollowNum     Number `json:"follow_num"`
	FansNum       Number `json:"fans_num"`
	IsFollow      Flag   `json:"is_follow"`
	IsFan         Flag   `json:"is_fan"`
}

type Version struct {
	Version   string `json:"version"`
	Changelog string `json:"changelog"`
	URL       string `json:"url"`
}

type Versions struct {
	Versions List[Version] `json:"versions"`
}

type VoteStatus struct {
	ID    Scalar `json:"viid"`
	Time  Scalar `json:"time"`
	Label string `json:"label"`
	Num   Number `json:"num"`
}

type VoteOption struct {
	ID    Scalar `json:"viid"`
	Label string `json:"label"`
	Num   Number `json:"num"`
}

type Vote struct {
	ID            Scalar           `json:"vid"`
	Title         string           `json:"title"`
	Start         Scalar           `json:"start"`
	End           Scalar           `json:"end"`
	UserCount     Number           `json:"user_count"`
	Type          Number           `json:"type"`
	Limit         Number           `json:"limit"`
	ArticleID     Scalar           `json:"aid"`
	IsEnd         Flag             `json:"is_end"`
	IsDeleted     Flag             `json:"is_deleted"`
	IsResultVoted Flag             `json:"is_result_voted"`
	Author        User             `json:"user"`
	VoteStatus    json.RawMessage  `json:"vote_status"`
	Voted         json.RawMessage  `json:"voted"`
	Options       List[VoteOption] `json:"options"`
	VoteCount     Number           `json:"vote_count"`
}

type Welcome struct {
	Version Number `json:"ver"`
	URL     string `json:"url"`
	Sign    string `json:"sign"`
}

type Page struct{ Number, Size int }
