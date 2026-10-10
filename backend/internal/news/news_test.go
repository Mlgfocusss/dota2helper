package news

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"d2helper/internal/clients/steam"
)

func render(src string) string { return Sanitize(ToHTML(src)) }

func TestBBCodeBasics(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"абзац и жирный", `[p]Hello [b]world[/b] and [i]you[/i][/p]`,
			`<p>Hello <strong>world</strong> and <em>you</em></p>`},
		{"заголовки", `[h1]Big[/h1][h3]Small[/h3]`, `<h2>Big</h2><h3>Small</h3>`},
		{"переносы строк", "Line one\nLine two", "Line one<br>Line two"},
		{"список с переносами", "[list]\n[*] One\n[*] Two\n[/list]", "<ul><li> One</li><li> Two</li></ul>"},
		{"список с [/*] и абзацами", `[list][*][p]A[/p][/*][*][p]B[/p][/*][/list]`,
			`<ul><li><p>A</p></li><li><p>B</p></li></ul>`},
		{"разворот", "[expand type=details]\nName\n[/expand]",
			"<details><summary>Подробнее</summary>Name</details>"},
		{"незнакомый тег", `[spoiler]secret[/spoiler]`, `secret`},
		{"лишний закрывающий", `text[/b]`, `text`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := render(tc.in); got != tc.want {
				t.Errorf("\nвход:     %q\nполучено: %q\nожидалось: %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestBBCodeLinksAndImages(t *testing.T) {
	got := render(`[url="https://www.dota2.com/patches/7.41f"]here[/url]`)
	if !strings.Contains(got, `href="https://www.dota2.com/patches/7.41f"`) ||
		!strings.Contains(got, `target="_blank"`) || !strings.Contains(got, "nofollow") ||
		!strings.Contains(got, "noreferrer") {
		t.Errorf("ссылка: %s", got)
	}

	got = render(`[img src="{STEAM_CLAN_LOC_IMAGE}/3703047/abc.png"][/img][img]{STEAM_CLAN_IMAGE}/1/x.jpg[/img]`)
	want := `<img src="https://clan.akamai.steamstatic.com/images/3703047/abc.png" alt="" loading="lazy">` +
		`<img src="https://clan.akamai.steamstatic.com/images/1/x.jpg" alt="" loading="lazy">`
	if got != want {
		t.Errorf("картинки:\n%s\n%s", got, want)
	}

	if got := render(`[url]https://example.com[/url]`); !strings.Contains(got, `>https://example.com</a>`) {
		t.Errorf("ссылка без атрибута: %s", got)
	}
}

func TestBBCodeIsSafe(t *testing.T) {
	cases := []string{
		`<script>alert(1)</script>`,
		`[url="javascript:alert(1)"]click[/url]`,
		`[img src="javascript:alert(1)"][/img]`,
		`[url="https://x.com/" onclick="alert(1)"]x[/url]`,
		`[p]<img src=x onerror=alert(1)>[/p]`,
	}
	// экранированный текст безопасен, проверяем только настоящие теги и атрибуты
	dangerous := regexp.MustCompile(`(?i)<script|<[^>]*\son[a-z]+\s*=|(?:href|src)\s*=\s*"?\s*javascript:`)
	for _, in := range cases {
		if got := render(in); dangerous.MatchString(got) {
			t.Errorf("опасная разметка прошла: %q → %q", in, got)
		}
	}
}

func TestExcerpt(t *testing.T) {
	src := `[p][img src="{STEAM_CLAN_LOC_IMAGE}/1/a.png"][/img][/p][p]Some time ago, a time now none of us can quite recall, we began to set our minds on matters of the monstrous.[/p]`
	got := Excerpt(src, 60)
	if strings.Contains(got, "[") || strings.Contains(got, "STEAM") {
		t.Errorf("в отрывке осталась разметка: %q", got)
	}
	if !strings.HasPrefix(got, "Some time ago") || !strings.HasSuffix(got, "…") || utf8.RuneCountInString(got) > 61 {
		t.Errorf("отрывок: %q", got)
	}
	if short := Excerpt("[p]Short news[/p]", 60); short != "Short news" {
		t.Errorf("короткий текст не обрезается: %q", short)
	}
}

func TestConvert(t *testing.T) {
	n := Convert(steam.NewsItem{
		GID: "1", Title: "7.41f Gameplay Patch", URL: "https://store.steampowered.com/news/1",
		Date: 1789430400, Contents: "[p]Patch notes [url=\"https://www.dota2.com/patches/7.41f\"]here[/url].[/p]",
	})
	if n.Date.Year() != 2026 || !strings.Contains(n.HTML, "<p>Patch notes") || n.Excerpt != "Patch notes here." {
		t.Errorf("Convert = %+v", n)
	}
}
