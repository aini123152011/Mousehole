package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"unicode/utf8"

	"mousehole/backend/internal/command"
)

func decodeRequest(w http.ResponseWriter, r *http.Request) (command.Request, error) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16*1024))
	if err != nil {
		return command.Request{}, err
	}
	if !utf8.Valid(body) {
		return command.Request{}, command.ErrInvalidRequest
	}
	// 标准解码器默认接受重复键和大小写不一致的字段，会使参数绑定产生歧义。
	// 这里先严格检查固定 JSON 结构，再映射为领域类型。
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err = uniqueObject(decoder, map[string]map[string]bool{"kind": nil, "payload": {"title": true}}); err != nil {
		return command.Request{}, err
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return command.Request{}, command.ErrInvalidRequest
	}
	var request command.Request
	if err = json.Unmarshal(body, &request); err != nil {
		return command.Request{}, err
	}
	return command.NormalizeRequest(request)
}

func uniqueObject(decoder *json.Decoder, fields map[string]map[string]bool) error {
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return command.ErrInvalidRequest
	}
	seen := make(map[string]bool)
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		children, allowed := fields[key]
		if !ok || !allowed || seen[key] {
			return command.ErrInvalidRequest
		}
		seen[key] = true
		if children != nil {
			nested := make(map[string]map[string]bool, len(children))
			for child := range children {
				nested[child] = nil
			}
			if err = uniqueObject(decoder, nested); err != nil {
				return err
			}
		} else {
			token, err = decoder.Token()
			if err != nil {
				return err
			}
			if _, ok = token.(string); !ok {
				return command.ErrInvalidRequest
			}
		}
	}
	if len(seen) != len(fields) {
		return command.ErrInvalidRequest
	}
	token, err = decoder.Token()
	if err != nil || token != json.Delim('}') {
		return command.ErrInvalidRequest
	}
	return nil
}
