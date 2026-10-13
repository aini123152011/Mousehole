//go:build integration

package scopecommand

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"mousehole/backend/internal/asset"
)

// 从原回执收集权威 ID，逐页核对三种资源无重/漏，cursor 不能改用其他列表或筛选。
func TestVersionPrecheckPaginationAndCursorBindings(t *testing.T) {
	f := database(t)
	ctx := context.Background()
	versions, checks := map[string]bool{}, map[string]bool{}
	for i := 0; i < 29; i++ {
		v := candidate(t, f)
		versions[v.ID] = true
		d := decision(t, accept(t, f, precheckRequest(v.ID)))
		checks[d.ID] = true
	}
	firstCursorVersion, firstCursorCheck := "", ""
	seenVersions, seenChecks := map[string]bool{}, map[string]bool{}
	cursor := ""
	for {
		p, e := f.s.ListVersions(ctx, f.actor, f.project, ListOptions{PageSize: 7, Cursor: cursor})
		if e != nil {
			t.Fatal(e)
		}
		if p.PageCount != strconv.Itoa(len(p.Items)) {
			t.Fatal("page_count应为当前页数量")
		}
		for _, v := range p.Items {
			if !versions[v.ID] || seenVersions[v.ID] {
				t.Fatal("授权页包含未知或重复ID")
			}
			seenVersions[v.ID] = true
		}
		if !p.HasMore {
			if p.NextCursor != nil {
				t.Fatal("末页cursor应为空")
			}
			break
		}
		if p.NextCursor == nil {
			t.Fatal("有下一页需cursor")
		}
		cursor = *p.NextCursor
		if firstCursorVersion == "" {
			firstCursorVersion = cursor
		}
	}
	cursor = ""
	for {
		p, e := f.s.ListPrechecks(ctx, f.actor, f.project, ListOptions{PageSize: 6, Cursor: cursor})
		if e != nil {
			t.Fatal(e)
		}
		if p.PageCount != strconv.Itoa(len(p.Items)) {
			t.Fatal("page_count应为当前页数量")
		}
		for _, d := range p.Items {
			if !checks[d.ID] || seenChecks[d.ID] {
				t.Fatal("决策页包含未知或重复ID")
			}
			seenChecks[d.ID] = true
		}
		if !p.HasMore {
			if p.NextCursor != nil {
				t.Fatal("末页cursor应为空")
			}
			break
		}
		if p.NextCursor == nil {
			t.Fatal("有下一页需cursor")
		}
		cursor = *p.NextCursor
		if firstCursorCheck == "" {
			firstCursorCheck = cursor
		}
	}
	if len(seenVersions) != len(versions) || len(seenChecks) != len(checks) {
		t.Fatal("大于一页的授权/预检查必须完整可翻查")
	}
	if _, e := f.s.ListVersions(ctx, f.actor, f.project, ListOptions{Cursor: firstCursorCheck}); !errors.Is(e, ErrInvalidRequest) {
		t.Fatal("决策cursor不能用于授权列表")
	}
	if _, e := f.s.ListPrechecks(ctx, f.actor, f.project, ListOptions{Cursor: firstCursorVersion}); !errors.Is(e, ErrInvalidRequest) {
		t.Fatal("授权cursor不能用于决策列表")
	}
	if _, e := f.s.ListVersions(ctx, f.actor, f.project, ListOptions{Cursor: firstCursorVersion + "A"}); !errors.Is(e, ErrInvalidRequest) {
		t.Fatal("授权cursor签名修改须拒绝")
	}
	if _, e := f.s.ListPrechecks(ctx, f.actor, f.project, ListOptions{Cursor: firstCursorCheck, Search: "秘密目标"}); !errors.Is(e, ErrInvalidRequest) {
		t.Fatal("决策不支持资产筛选")
	}
	accept(t, f, request("import_assets", asset.Import{Source: "manual", Entries: []asset.Input{{Kind: "domain", Value: "one.invalid"}, {Kind: "domain", Value: "two.invalid"}, {Kind: "ip", Value: "10.42.0.9"}}}))
	p, e := f.s.ListAssets(ctx, f.actor, f.project, ListOptions{PageSize: 1, Kind: "domain"})
	if e != nil || p.NextCursor == nil {
		t.Fatal("合成资产需生成有筛选cursor")
	}
	for _, o := range []ListOptions{{Cursor: *p.NextCursor, Kind: "ip"}, {Cursor: *p.NextCursor, Kind: "domain", Search: "one"}, {Cursor: firstCursorVersion}} {
		if _, e = f.s.ListAssets(ctx, f.actor, f.project, o); !errors.Is(e, ErrInvalidRequest) {
			t.Fatal("资产cursor不能换筛选或资源")
		}
	}
}
