package news

import (
	"html"
	"net/url"
	"regexp"
	"strings"
)

// Steam подставляет адрес CDN картинок через плейсхолдеры.
const clanImageBase = "https://clan.akamai.steamstatic.com/images"

var (
	tagRe          = regexp.MustCompile(`\[(/?)([a-zA-Z0-9*]+)((?:=|\s)[^\]]*)?\]`)
	placeholderRe  = regexp.MustCompile(`\{STEAM_CLAN(?:_LOC)?_IMAGE\}`)
	attrRe         = regexp.MustCompile(`([a-zA-Z_]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"']+))`)
	bareURLRe      = regexp.MustCompile(`(?is)\[url\]([^\[]+)\[/url\]`)
	bareImgRe      = regexp.MustCompile(`(?is)\[img\]([^\[]+)\[/img\]`)
	blockTag       = `(?:p|ul|ol|li|h2|h3|h4|blockquote|pre|details|summary|hr|table|tr|td|th)`
	brAfterBlockRe = regexp.MustCompile(`(<` + `/?` + blockTag + `\b[^>]*>)(?:\s*<br>)+`)
	brBeforeBlock  = regexp.MustCompile(`(?:<br>\s*)+(</?` + blockTag + `\b)`)
	manyBrRe       = regexp.MustCompile(`(?:<br>\s*){3,}`)
	emptyPRe       = regexp.MustCompile(`<p>\s*</p>`)
)

// simple — теги, которые превращаются в HTML один к одному.
var simple = map[string]string{
	"p": "p", "b": "strong", "i": "em", "u": "u", "s": "s", "strike": "s",
	"h1": "h2", "h2": "h2", "h3": "h3", "h4": "h4", "quote": "blockquote",
	"table": "table", "tr": "tr", "td": "td", "th": "th",
	"list": "ul", "olist": "ol",
}

// ToHTML переводит BBCode анонсов Steam в HTML. Результат ещё нужно
// пропустить через Sanitize — здесь только разметка.
func ToHTML(src string) string {
	src = bareURLRe.ReplaceAllString(src, `[url="$1"]$1[/url]`)
	src = bareImgRe.ReplaceAllString(src, `[img src="$1"][/img]`)

	var b strings.Builder
	var stack []string // открытые HTML‑теги
	liOpen := []bool{} // открыт ли <li> в каждом списке

	closeTo := func(tag string) {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i] != tag {
				continue
			}
			for j := len(stack) - 1; j >= i; j-- {
				b.WriteString("</" + stack[j] + ">")
				if stack[j] == "ul" || stack[j] == "ol" {
					liOpen = liOpen[:len(liOpen)-1]
				}
			}
			stack = stack[:i]
			return
		}
	}
	open := func(tag, attrs string) {
		b.WriteString("<" + tag + attrs + ">")
		stack = append(stack, tag)
	}

	pos := 0
	for _, m := range tagRe.FindAllStringSubmatchIndex(src, -1) {
		writeText(&b, src[pos:m[0]])
		pos = m[1]
		closing := m[3] > m[2]
		name := strings.ToLower(src[m[4]:m[5]])
		attr := ""
		if m[6] >= 0 {
			attr = src[m[6]:m[7]]
		}

		switch {
		case name == "*":
			if closing {
				closeTo("li")
				if n := len(liOpen); n > 0 {
					liOpen[n-1] = false
				}
				continue
			}
			if n := len(liOpen); n > 0 {
				if liOpen[n-1] {
					closeTo("li")
				}
				open("li", "")
				liOpen[n-1] = true
			}
		case simple[name] != "":
			tag := simple[name]
			if closing {
				closeTo(tag)
				continue
			}
			open(tag, "")
			if tag == "ul" || tag == "ol" {
				liOpen = append(liOpen, false)
			}
		case name == "url":
			if closing {
				closeTo("a")
				continue
			}
			href := attrValue(attr, "")
			if href == "" {
				continue
			}
			open("a", ` href="`+html.EscapeString(href)+`"`)
		case name == "img":
			if closing {
				continue
			}
			if srcURL := imageURL(attrValue(attr, "src")); srcURL != "" {
				b.WriteString(`<img src="` + html.EscapeString(srcURL) + `" alt="" loading="lazy">`)
			}
		case name == "expand":
			if closing {
				closeTo("details")
				continue
			}
			open("details", "")
			b.WriteString("<summary>Подробнее</summary>")
		case name == "code":
			if closing {
				closeTo("pre")
				continue
			}
			open("pre", "")
		case name == "hr":
			if !closing {
				b.WriteString("<hr>")
			}
		case name == "previewyoutube":
			if closing {
				continue
			}
			id, _, _ := strings.Cut(attrValue(attr, ""), ";")
			if id != "" {
				b.WriteString(`<p><a href="https://www.youtube.com/watch?v=` + url.QueryEscape(id) + `">Смотреть видео на YouTube</a></p>`)
			}
		default:
			// незнакомые теги ([spoiler], [noparse]…) убираем, текст внутри остаётся
		}
	}
	writeText(&b, src[pos:])
	for i := len(stack) - 1; i >= 0; i-- {
		b.WriteString("</" + stack[i] + ">")
	}
	return tidy(b.String())
}

func writeText(b *strings.Builder, text string) {
	if text == "" {
		return
	}
	b.WriteString(strings.ReplaceAll(html.EscapeString(text), "\n", "<br>"))
}

// tidy убирает переносы строк, которые в BBCode стоят вокруг блочных тегов.
func tidy(s string) string {
	for {
		next := brAfterBlockRe.ReplaceAllString(s, "$1")
		next = brBeforeBlock.ReplaceAllString(next, "$1")
		next = manyBrRe.ReplaceAllString(next, "<br><br>")
		next = emptyPRe.ReplaceAllString(next, "")
		if next == s {
			return strings.TrimSpace(s)
		}
		s = next
	}
}

// attrValue достаёт значение атрибута тега: [url="x"], [url=x], [img src="x"].
// key == "" — значение сразу после «=».
func attrValue(attr, key string) string {
	attr = strings.TrimSpace(attr)
	if key == "" {
		v := strings.TrimPrefix(attr, "=")
		return strings.Trim(strings.TrimSpace(v), `"'`)
	}
	for _, m := range attrRe.FindAllStringSubmatch(attr, -1) {
		if strings.EqualFold(m[1], key) {
			return m[2] + m[3] + m[4]
		}
	}
	// [img="x"]
	if strings.HasPrefix(attr, "=") {
		return strings.Trim(strings.TrimPrefix(attr, "="), `"' `)
	}
	return ""
}

func imageURL(raw string) string {
	u := strings.TrimSpace(placeholderRe.ReplaceAllString(raw, clanImageBase))
	if strings.HasPrefix(u, "http://") {
		u = "https://" + strings.TrimPrefix(u, "http://")
	}
	if !strings.HasPrefix(u, "https://") {
		return ""
	}
	return u
}

var (
	spaceRe         = regexp.MustCompile(`\s+`)
	spaceBeforeMark = regexp.MustCompile(`\s+([.,;:!?)])`)
	inlineTagRe     = regexp.MustCompile(`(?i)\[/?(?:b|i|u|s|strike|url)(?:=[^\]]*)?\]`)
)

// PlainText — текст анонса без разметки и картинок.
func PlainText(src string) string {
	src = bareImgRe.ReplaceAllString(src, " ")
	// строчные теги убираем без следа, блочные отделяем пробелом
	text := inlineTagRe.ReplaceAllString(src, "")
	text = tagRe.ReplaceAllString(text, " ")
	text = spaceRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(spaceBeforeMark.ReplaceAllString(text, "$1"))
}

// Excerpt — начало текста для списка новостей, обрезанное по границе слова.
func Excerpt(src string, limit int) string {
	text := []rune(PlainText(src))
	if len(text) <= limit {
		return string(text)
	}
	cut := text[:limit]
	if i := strings.LastIndexAny(string(cut), " ,.;:!?"); i > limit/2 {
		cut = []rune(string(cut)[:i])
	}
	return strings.TrimRight(string(cut), " ,.;:!?") + "…"
}
