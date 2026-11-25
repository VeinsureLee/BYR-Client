package forum

// Post is the validated semantic snapshot sent by the composer.
type Post struct {
	Board, Title, Content, ReplyID     string
	Anonymous, Signature, Email, Outgo bool
}

// Search is the protocol-level filter used by the shared application search
// use case. Endpoint fields remain explicit until a wire-to-domain conversion
// is introduced in a later compatibility batch.
type Search struct {
	Board, Title, Title2, ExcludeTitle, Author, Day, Marked, Attachment, Order string
	Page                                                                       Page
}

// WebLogin is the result of the explicit HTML login diagnostic; it never
// replaces the OAuth session used for normal API calls.
type WebLogin struct {
	State   Scalar `json:"ajax_st"`
	Code    Scalar `json:"ajax_code"`
	Message string `json:"ajax_msg"`
}

func (w WebLogin) OK() bool { return w.State == "1" }
