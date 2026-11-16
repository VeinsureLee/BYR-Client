package forum

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Protocol adapters stay here; UI and use cases deal with typed values.
type Scalar string

func (s *Scalar) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		*s = ""
		return nil
	}
	var v string
	if json.Unmarshal(data, &v) == nil {
		*s = Scalar(v)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*s = Scalar(n.String())
		return nil
	}
	// The server types loosely; fields like a reaction's status come back as
	// booleans on some responses. Absorb them instead of failing the page.
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*s = Scalar(strconv.FormatBool(b))
		return nil
	}
	return fmt.Errorf("expected string, number or boolean: %w", json.Unmarshal(data, &n))
}
func (s Scalar) String() string { return string(s) }

type Number int

func (n *Number) UnmarshalJSON(data []byte) error {
	var s Scalar
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		*n = 0
		return nil
	}
	v, err := strconv.Atoi(string(s))
	if err == nil {
		*n = Number(v)
	}
	return err
}

type Flag bool

func (f *Flag) UnmarshalJSON(data []byte) error {
	s := strings.ToLower(strings.Trim(string(data), "\" \t\r\n"))
	switch s {
	case "true", "1":
		*f = true
	case "false", "0", "null", "":
		*f = false
	default:
		return fmt.Errorf("invalid boolean %q", s)
	}
	return nil
}

type List[T any] []T

func (l *List[T]) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*l = nil
		return nil
	}
	if len(data) > 0 && data[0] == '[' {
		var v []T
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		*l = v
		return nil
	}
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*l = []T{v}
	return nil
}
func (u *User) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*u = User{ID: Scalar(s), UserName: s}
		return nil
	}
	type plain User
	return json.Unmarshal(data, (*plain)(u))
}

// Subsection lists contain references such as ["1", "2"] as well as full
// section objects. Resolve both at the protocol boundary instead of rejecting
// an otherwise valid board list because of one reference.
func (s *Section) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] != '{' && !bytes.Equal(data, []byte("null")) {
		var name Scalar
		if err := json.Unmarshal(data, &name); err != nil {
			return err
		}
		*s = Section{Name: name}
		return nil
	}
	type plain Section
	return json.Unmarshal(data, (*plain)(s))
}
func (u User) DisplayName() string {
	if strings.TrimSpace(u.UserName) != "" {
		return u.UserName
	}
	if u.ID != "" {
		return string(u.ID)
	}
	return "未知用户"
}
func (u User) Anonymous() bool { return strings.HasPrefix(string(u.ID), "IWhisper") }
func (c *Collection) UnmarshalJSON(data []byte) error {
	type plain Collection
	v := struct {
		*plain
		Legacy List[CollectionEntry] `json:"collection"`
	}{plain: (*plain)(c)}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	if c.Entries == nil {
		c.Entries = v.Legacy
	}
	return nil
}
