package application

import (
	"net/url"
	"strings"
)

type URLBuilder struct {
	baseURL string
}

func NewURLBuilder(baseURL string) URLBuilder {
	return URLBuilder{baseURL: strings.TrimRight(baseURL, "/")}
}

func (b URLBuilder) Confirm(token string) string {
	return b.baseURL + "/api/confirm/" + url.PathEscape(token)
}

func (b URLBuilder) Unsubscribe(token string) string {
	return b.baseURL + "/api/unsubscribe/" + url.PathEscape(token)
}
