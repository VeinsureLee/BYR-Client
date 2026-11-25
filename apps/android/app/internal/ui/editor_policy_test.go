package ui

import (
	"testing"

	"gioui.org/io/key"
)

func TestAndroidEditableFieldPolicies(t *testing.T) {
	server := newInteractionServer(t)
	a := interactionApp(t, server.server.URL)
	defer a.Close()

	if !a.username.SingleLine || a.username.InputHint != key.HintEmail {
		t.Fatalf("username policy = singleLine:%t hint:%v", a.username.SingleLine, a.username.InputHint)
	}
	if !a.password.SingleLine || a.password.Submit || a.password.InputHint != key.HintPassword {
		t.Fatalf("password policy = singleLine:%t submit:%t hint:%v", a.password.SingleLine, a.password.Submit, a.password.InputHint)
	}
	if !a.searchEdit.SingleLine || a.searchEdit.Submit || a.searchEdit.InputHint != key.HintAny {
		t.Fatalf("search policy = singleLine:%t submit:%t hint:%v", a.searchEdit.SingleLine, a.searchEdit.Submit, a.searchEdit.InputHint)
	}
	a.setSearchMode("user")
	if a.searchEdit.InputHint != key.HintEmail {
		t.Fatalf("user-search hint = %v, want email/identifier hint", a.searchEdit.InputHint)
	}
	a.setSearchMode("all")
	if a.searchEdit.InputHint != key.HintAny {
		t.Fatalf("all-site-search hint = %v, want general text", a.searchEdit.InputHint)
	}

	if !a.forwardIn.SingleLine || a.forwardIn.InputHint != key.HintEmail ||
		!a.blacklistIn.SingleLine || a.blacklistIn.InputHint != key.HintEmail {
		t.Fatalf("identifier policies: forward=%t/%v blacklist=%t/%v",
			a.forwardIn.SingleLine, a.forwardIn.InputHint, a.blacklistIn.SingleLine, a.blacklistIn.InputHint)
	}
	if !a.scoreIn.SingleLine || a.scoreIn.InputHint != key.HintNumeric {
		t.Fatalf("bet score policy = singleLine:%t hint:%v", a.scoreIn.SingleLine, a.scoreIn.InputHint)
	}

	a.stack = []Route{{Page: PageHome}}
	a.openCompose("Go", "", "new")
	if !a.compose.titleEditor.SingleLine || a.compose.titleEditor.Submit ||
		a.compose.titleEditor.InputHint != key.HintAny || a.compose.contentEditor.SingleLine ||
		a.compose.contentEditor.Submit || a.compose.contentEditor.InputHint != key.HintAny {
		t.Fatalf("compose title/body policies = %t/%t/%v and %t/%t/%v",
			a.compose.titleEditor.SingleLine, a.compose.titleEditor.Submit, a.compose.titleEditor.InputHint,
			a.compose.contentEditor.SingleLine, a.compose.contentEditor.Submit, a.compose.contentEditor.InputHint)
	}
	if !a.compose.linkURL.SingleLine || a.compose.linkURL.Submit || a.compose.linkURL.InputHint != key.HintURL ||
		!a.compose.linkText.SingleLine || a.compose.linkText.Submit || a.compose.linkText.InputHint != key.HintAny {
		t.Fatalf("link input policies = %t/%t/%v and %t/%t/%v",
			a.compose.linkURL.SingleLine, a.compose.linkURL.Submit, a.compose.linkURL.InputHint,
			a.compose.linkText.SingleLine, a.compose.linkText.Submit, a.compose.linkText.InputHint)
	}

	a.stack = []Route{{Page: PageMessages, Sub: "mail", Name: "contacts"}}
	a.openMailCompose("bob", "", "", "send")
	if !a.mailEditor.toEditor.SingleLine || a.mailEditor.toEditor.Submit || a.mailEditor.toEditor.InputHint != key.HintEmail ||
		!a.mailEditor.titleEditor.SingleLine || a.mailEditor.titleEditor.Submit || a.mailEditor.titleEditor.InputHint != key.HintAny ||
		a.mailEditor.contentEditor.SingleLine || a.mailEditor.contentEditor.Submit || a.mailEditor.contentEditor.InputHint != key.HintAny {
		t.Fatalf("mail editor policies = recipient:%t/%t/%v title:%t/%t/%v body:%t/%t/%v",
			a.mailEditor.toEditor.SingleLine, a.mailEditor.toEditor.Submit, a.mailEditor.toEditor.InputHint,
			a.mailEditor.titleEditor.SingleLine, a.mailEditor.titleEditor.Submit, a.mailEditor.titleEditor.InputHint,
			a.mailEditor.contentEditor.SingleLine, a.mailEditor.contentEditor.Submit, a.mailEditor.contentEditor.InputHint)
	}
}
