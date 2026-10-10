package party

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"d2helper/internal/store"
)

// Курсор — позиция последней показанной анкеты: «микросекунды подъёма:id автора».
// Одной метки времени мало: подъёмы в одну микросекунду возможны.

func encodeCursor(p store.PartyProfile) string {
	raw := fmt.Sprintf("%d:%d", p.BumpedAt.UnixMicro(), p.UserID)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (*store.PartyCursor, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrBadCursor
	}
	ts, id, ok := strings.Cut(string(raw), ":")
	if !ok {
		return nil, ErrBadCursor
	}
	micro, err1 := strconv.ParseInt(ts, 10, 64)
	userID, err2 := strconv.ParseInt(id, 10, 64)
	if err1 != nil || err2 != nil || userID < 1 {
		return nil, ErrBadCursor
	}
	return &store.PartyCursor{BumpedAt: time.UnixMicro(micro), UserID: userID}, nil
}
