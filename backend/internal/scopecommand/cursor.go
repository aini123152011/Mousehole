package scopecommand

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"mousehole/backend/internal/command"
)

type cursorData struct {
	FilterHash string `json:"filter_hash"`
	At         string `json:"at"`
	ID         string `json:"id"`
}

func filterHash(project, resource string, o ListOptions) string {
	encoded, _ := json.Marshal([]string{project, resource, o.Kind, o.Search, "created_at_desc_id_desc"})
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
func (s *Service) cursor(project, resource string, o ListOptions, at, id string) string {
	data, _ := json.Marshal(cursorData{filterHash(project, resource, o), at, id})
	mac := hmac.New(sha256.New, s.cursorKey)
	_, _ = mac.Write(data)
	return base64.RawURLEncoding.EncodeToString(data) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (s *Service) parseCursor(project, resource string, o ListOptions) (ListOptions, *cursorData, error) {
	if o.PageSize == 0 {
		o.PageSize = 25
	}
	if o.PageSize < 1 || o.PageSize > 100 || utf8.RuneCountInString(o.Search) > 200 || !utf8.ValidString(o.Search) {
		return o, nil, ErrInvalidRequest
	}
	if o.Kind != "" && o.Kind != "domain" && o.Kind != "ip" && o.Kind != "url" {
		return o, nil, ErrInvalidRequest
	}
	if o.Cursor == "" {
		return o, nil, nil
	}
	if len(o.Cursor) > 1024 {
		return o, nil, ErrInvalidRequest
	}
	parts := strings.Split(o.Cursor, ".")
	if len(parts) != 2 {
		return o, nil, ErrInvalidRequest
	}
	data, e := base64.RawURLEncoding.DecodeString(parts[0])
	sig, se := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil || se != nil {
		return o, nil, ErrInvalidRequest
	}
	mac := hmac.New(sha256.New, s.cursorKey)
	_, _ = mac.Write(data)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return o, nil, ErrInvalidRequest
	}
	var c cursorData
	if json.Unmarshal(data, &c) != nil || c.FilterHash != filterHash(project, resource, o) || !command.ValidID(c.ID) {
		return o, nil, ErrInvalidRequest
	}
	if _, e = time.Parse(time.RFC3339Nano, c.At); e != nil {
		return o, nil, ErrInvalidRequest
	}
	return o, &c, nil
}
func escapedSearch(value string) string {
	return strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value)
}
