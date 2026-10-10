package news

import (
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

// policy — белый список HTML для текста новостей: только разметка, ссылки
// http(s) и картинки https. Скрипты, стили и обработчики событий вырезаются.
var policy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements(
		"p", "br", "strong", "em", "u", "s", "h2", "h3", "h4", "ul", "ol", "li",
		"blockquote", "pre", "code", "hr", "details", "summary", "table", "tr", "td", "th",
	)
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("https", "http")
	p.RequireParseableURLs(true)
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	p.AllowAttrs("src").Matching(regexp.MustCompile(`^https://`)).OnElements("img")
	p.AllowAttrs("alt").OnElements("img")
	p.AllowAttrs("loading").Matching(regexp.MustCompile(`^lazy$`)).OnElements("img")
	return p
}()

// Sanitize оставляет в HTML только безопасные теги и атрибуты.
func Sanitize(html string) string {
	return policy.Sanitize(html)
}
