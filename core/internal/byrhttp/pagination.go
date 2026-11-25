package byrhttp

import (
	"errors"
	"net/url"
	"strconv"
)

func pageQuery(page Page) (url.Values, error) {
	if page.Number == 0 {
		page.Number = 1
	}
	if page.Size == 0 {
		page.Size = 50
	}
	if page.Number < 1 || page.Size < 1 || page.Size > 100 {
		return nil, errors.New("invalid forum pagination")
	}
	return url.Values{"page": {strconv.Itoa(page.Number)}, "count": {strconv.Itoa(page.Size)}}, nil
}
