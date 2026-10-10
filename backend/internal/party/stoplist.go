package party

import (
	"regexp"
	"strings"
	"unicode"
)

// Простой стоп-лист для поля «О себе» (SPEC 4.4): ссылки и грубый мат.
// Это защита от очевидного спама, а не полноценная модерация — остальное решают жалобы.

var linkRe = regexp.MustCompile(
	`(?i)(https?://|www\.|t\.me/|discord\.gg|discord\.com/invite|vk\.com|` +
		`\b[a-z0-9-]+\.(ru|com|net|org|gg|me|io|tv|xyz|su|ly|cc|info|shop|store)\b)`)

// корни ругательств: слово блокируется, если начинается с корня,
// возможно после приставки («наебал», «выебал»)
var profanityStems = []string{
	"хуй", "хуе", "хуя", "хую", "хуи", "хуё",
	"пизд", "пезд",
	"ебат", "ебан", "ебал", "ебну", "ебуч", "ебло", "ебыр", "ебец",
	"блят", "бляд", "блеат",
	"сука", "суки", "сучк", "сучар",
	"мудак", "мудил", "мудач", "мудоз",
	"пидор", "пидар", "пидр",
	"гандон", "гондон",
	"залуп", "манда",
	"долбоеб", "долбоёб", "дебил", "дегенерат",
}

var profanityPrefixes = []string{
	"по", "на", "за", "вы", "у", "при", "раз", "рас", "под", "от", "до", "с", "об", "про",
	"пере", "из", "ис", "в", "отъ", "объ", "съ", "недо", "пре",
}

// ContainsForbidden сообщает, есть ли в тексте ссылка или грубая лексика.
func ContainsForbidden(text string) bool {
	if linkRe.MatchString(text) {
		return true
	}
	lower := strings.ToLower(text)
	for _, token := range strings.FieldsFunc(lower, func(r rune) bool { return !unicode.IsLetter(r) }) {
		if isProfane(token) {
			return true
		}
	}
	return false
}

func isProfane(token string) bool {
	if hasStem(token) {
		return true
	}
	for _, p := range profanityPrefixes {
		if rest, ok := strings.CutPrefix(token, p); ok && rest != "" && hasStem(rest) {
			return true
		}
	}
	return false
}

func hasStem(s string) bool {
	for _, stem := range profanityStems {
		if strings.HasPrefix(s, stem) {
			return true
		}
	}
	return false
}
