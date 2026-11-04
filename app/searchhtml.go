package main

import (
	"context"
	"encoding/binary"
	"errors"
	"math/rand"
	"net/url"
	"sort"
	"strings"
	"time"
)

type searchState struct {
	area         string
	tag          string
	language     string
	orderby      string
	orderbykey   string
	orderbydir   string
	orderApplied bool
}

// normalizeSearchTerm は hitomi 正典の `_` → space（results.js:30、search.js:653）の mirror。
// D-10 正典確認 2026-11-04: galleriesindex の index key は space 形（live 実測 `big breasts`
// key 92ef4857 は存在し `big_breasts` key 95e737a6 は不在）。hashTerm へ渡す term は space 形。
func normalizeSearchTerm(term string) string {
	return strings.ToLower(strings.ReplaceAll(term, "_", " "))
}

func isOrderTerm(term string) bool {
	if !strings.Contains(term, ":") {
		return false
	}
	return strings.HasPrefix(term, "orderby") || strings.HasPrefix(term, "sortby")
}

func isLanguageTerm(term string) bool {
	if i := strings.Index(term, ":"); i >= 0 {
		return term[:i] == "language"
	}
	return false
}

func applyOrderTerm(state *searchState, term string) {
	sides := strings.Split(term, ":")
	if len(sides) < 2 {
		return
	}
	left, right := sides[0], sides[1]
	switch left {
	case "sortbykey", "orderbykey":
		state.orderbykey = right
	case "orderby", "sortby":
		switch right {
		case "popular", "popularity":
			state.orderby = "popular"
		case "date":
			state.orderby = "date"
		case "datepublished":
			state.orderby = "date"
			state.orderbykey = "published"
		case "random", "rand":
			state.orderbydir = "random"
		default:
			state.orderbykey = right
		}
	case "sortbydirection", "orderbydirection":
		state.orderbydir = right
	}
}

func decodeNozomiIDs(body []byte) ([]int, error) {
	out := []int{}
	for i := 0; i+4 <= len(body); i += 4 {
		out = append(out, int(int32(binary.BigEndian.Uint32(body[i:i+4]))))
	}
	return out, nil
}

func copyInts(ids []int) []int {
	out := make([]int, len(ids))
	copy(out, ids)
	return out
}

// resolvedOrder は results.js（89-95）の orderbykey 既定確定の mirror（D-09）:
// undefined 時 popular → 'year'、else 'added'。既定確定は term parse 後・fetch 前。
func resolvedOrder(state *searchState) (string, string) {
	orderby := state.orderby
	if orderby == "" {
		orderby = "date"
	}
	key := state.orderbykey
	if key == "" {
		if orderby == "popular" {
			key = "year"
		} else {
			key = "added"
		}
	}
	return orderby, key
}

// SearchNozomiParts は hitomi `nozomi_address_from_state`（search.js 612-624・D-09 実測）を mirror。
// `orderby != 'date' || orderbykey == 'published'` のとき order-key dataset（例 `/n/tag/popular/year/
// female:yuri-all.nozomi`・area 'all' は `/n/popular/year-all.nozomi`）を選ぶ。集合順序はその dataset
// 自身の順序で変わる（client sort は正典にない。`order_results` は方向のみ）。dataset 選択が変わるため
// cache key にも order を含める（date dataset と混ざらない）。
func (c *Client) SearchNozomiParts(state *searchState) ([]int, error) {
	orderby, orderKey := resolvedOrder(state)
	cacheKey := state.area + "|" + state.tag + "|" + state.language
	path := nozomiPath(state.area, state.tag, state.language)
	if orderby != "date" || orderKey == "published" {
		cacheKey = state.area + "|" + orderby + "|" + orderKey + "|" + state.tag + "|" + state.language
		if state.area == "" {
			path = "/n/" + orderby + "/" + url.PathEscape(orderKey+"-"+state.language) + ".nozomi"
		} else {
			path = "/n/" + url.PathEscape(state.area) + "/" + url.PathEscape(orderby) + "/" + url.PathEscape(orderKey) + "/" + url.PathEscape(state.tag+"-"+state.language) + ".nozomi"
		}
	}

	c.searchCacheMu.Lock()
	if entry, ok := c.searchCache[cacheKey]; ok && time.Since(entry.fetchedAt) < searchCacheTTL {
		entry.lastAccess = time.Now()
		c.searchCacheMu.Unlock()
		return copyInts(entry.ids), nil
	}
	c.searchCacheMu.Unlock()

	body, err := c.get(context.Background(), "https://ltn."+hitomiDomain+path, [2]int64{-1, -1})
	if err != nil {
		return nil, err
	}
	ids, err := decodeNozomiIDs(body)
	if err != nil {
		return nil, err
	}

	c.searchCacheMu.Lock()
	if c.searchCache == nil {
		c.searchCache = map[string]*searchCacheEntry{}
	}
	now := time.Now()
	c.putSearchCacheLocked(cacheKey, &searchCacheEntry{ids: copyInts(ids), fetchedAt: now, lastAccess: now})
	c.searchCacheMu.Unlock()

	return ids, nil
}

func (c *Client) termIDs(state *searchState, term string) ([]int, error) {
	term = normalizeSearchTerm(term)
	term = strings.TrimPrefix(term, "-")

	if i := strings.Index(term, ":"); i >= 0 {
		left, right := strings.ToLower(term[:i]), term[i+1:]
		switch left {
		case "female", "male":
			state.area = "tag"
			state.tag = SanitizeTerm(term)
		case "language":
			state.language = strings.ToLower(right)
			// C-04: `language:` は lang filter そのもの。前置 term で fetch 済みの
			// 同じ area/tag を新 language で再 fetch せず、自前 dataset（index-<lang>）を 1 回だけ fetch する。
			dataset := *state
			dataset.area = ""
			dataset.tag = "index"
			return c.SearchNozomiParts(&dataset)
		default:
			state.area = left
			state.tag = SanitizeTerm(right)
		}
		return c.SearchNozomiParts(state)
	}
	// A-04: plain term は lang を適用しない（hitomi 正典準拠・意図的）。galleriesindex の
	// `galleries` は言語キーを持たず key = hash_term(term) のみ。language は `language:` term の
	// dataset としてのみ結果へ入る。live 確認 2026-11-04・テストで固定。
	return c.IndexIDs("galleries", term)
}

func intersectIDs(ordered, ids []int) []int {
	set := map[int]bool{}
	for _, id := range ids {
		set[id] = true
	}
	out := []int{}
	for _, id := range ordered {
		if set[id] {
			out = append(out, id)
		}
	}
	return out
}

func excludeIDs(ordered, ids []int) []int {
	set := map[int]bool{}
	for _, id := range ids {
		set[id] = true
	}
	out := []int{}
	for _, id := range ordered {
		if !set[id] {
			out = append(out, id)
		}
	}
	return out
}

func unionIDs(groups [][]int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, ids := range groups {
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

func filterNonEmptyOrGroups(groups [][]string) [][]string {
	out := [][]string{}
	for _, group := range groups {
		if len(group) > 0 {
			out = append(out, group)
		}
	}
	return out
}

func applySearchOrder(ids []int, state *searchState) {
	if state.orderApplied {
		return
	}
	state.orderApplied = true
	switch state.orderbydir {
	case "asc", "ascending":
		for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
			ids[i], ids[j] = ids[j], ids[i]
		}
	case "rand", "random":
		rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	}
}

func searchPageSlice(ids []int, page, limit int) []int {
	if limit < 0 {
		return ids
	}
	start := (page - 1) * galleriesPerPage
	if start > len(ids) {
		return nil
	}
	end := start + galleriesPerPage
	if end > len(ids) {
		end = len(ids)
	}
	out := ids[start:end]
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// SearchHTML は hitomi の results.js 相当。plain term は galleriesindex の 1 term hash exact、
// colon term は nozomi。`language:` は state の language を変え、最初の term fetch へ反映する（C-04）。
// 前置 term で fetch 済みの同じ area/tag を新 language で再 fetch しない。
// `search.html?furry` は部分一致ではなく、index に exact key がある term だけ返す。
func (c *Client) SearchHTML(query, lang string, page, limit int) ([]int, int, error) {
	terms := []string{}
	for _, rawTerm := range strings.Fields(query) {
		terms = append(terms, normalizeSearchTerm(rawTerm))
	}
	if len(terms) == 0 {
		return nil, 0, errors.New("query has no searchable term")
	}

	state := searchState{area: "", tag: "index", language: orDefault(lang, "all"), orderby: "date", orderbydir: "desc"}
	var positive, negative []string
	var orGroups [][]string

	for i, term := range terms {
		if isOrderTerm(term) {
			applyOrderTerm(&state, term)
			continue
		}
		if term == "or" {
			continue
		}
		orPrev := i > 0 && terms[i-1] == "or"
		orNext := i+1 < len(terms) && terms[i+1] == "or"
		if orPrev || orNext {
			if len(orGroups) == 0 {
				orGroups = append(orGroups, []string{})
			}
			orGroups[len(orGroups)-1] = append(orGroups[len(orGroups)-1], term)
			if !orNext {
				orGroups = append(orGroups, []string{})
			}
			continue
		}
		if strings.HasPrefix(term, "-") {
			negative = append(negative, term[1:])
		} else {
			positive = append(positive, term)
		}
	}

	orGroups = filterNonEmptyOrGroups(orGroups)

	if len(positive) == 0 && len(orGroups) == 0 {
		return nil, 0, errors.New("query has no positive term")
	}

	// D-09（results.js 89-95 mirror）: orderbykey 既定確定を fetch 前に行う（popular → 'year'、else 'added'）。
	if state.orderbykey == "" {
		if state.orderby == "popular" {
			state.orderbykey = "year"
		} else {
			state.orderbykey = "added"
		}
	}

	sort.SliceStable(positive, func(i, j int) bool {
		if strings.Contains(positive[i], ":") && !strings.Contains(positive[j], ":") {
			return true
		}
		if !strings.Contains(positive[i], ":") && strings.Contains(positive[j], ":") {
			return false
		}
		return false
	})

	// C-04: language term を fetch 前に pre-scan し、state.language を最初の term fetch
	// へ反映する。同じ tag の language 違い fetch を 1 回に固定する（実 data の
	// language-scoped dataset は all dataset の subset なので intersection 結果は不変）。
	for _, term := range positive {
		if isLanguageTerm(term) {
			if right := term[strings.Index(term, ":")+1:]; right != "" {
				state.language = right
			}
			break
		}
	}

	var results []int
	orGroupOffset := 0
	positiveStart := 1
	if len(positive) > 0 {
		// D-09（results.js 114 mirror・正典実測）: 最初の positive term が plain で
		// orderbykey != 'added' のとき集合順序の起点は order dataset。term membership は
		// 後段の positive filter で intersection（順序保持）なので positive[0] は loop 側で処理する。
		if !strings.Contains(positive[0], ":") && state.orderbykey != "added" {
			positiveStart = 0
			ids, err := c.SearchNozomiParts(&state)
			if err != nil {
				return nil, 0, err
			}
			results = ids
			applyOrderForInitial(results, &state)
		} else {
			ids, err := c.termIDs(&state, positive[0])
			if err != nil {
				return nil, 0, err
			}
			results = ids
			applyOrderForInitial(results, &state)
		}
	} else {
		var groupIDs [][]int
		for _, term := range orGroups[0] {
			ids, err := c.termIDs(&state, term)
			if err != nil {
				return nil, 0, err
			}
			groupIDs = append(groupIDs, ids)
		}
		results = unionIDs(groupIDs)
		applySearchOrder(results, &state)
		orGroupOffset = 1
	}

	for _, group := range orGroups[orGroupOffset:] {
		set := map[int]bool{}
		for _, term := range group {
			ids, err := c.termIDs(&state, term)
			if err != nil {
				return nil, 0, err
			}
			for _, id := range ids {
				set[id] = true
			}
		}
		next := []int{}
		for _, id := range results {
			if set[id] {
				next = append(next, id)
			}
		}
		results = next
	}

	if len(positive) > positiveStart {
		for _, term := range positive[positiveStart:] {
			if isLanguageTerm(term) {
				// C-04: pre-scan で state.language は fetch 済み。この term の dataset は
				// fetch 済みの language-scoped dataset と同一（subset 保証で intersection no-op）なので
				// 同じ area/tag を再 fetch しない。
				continue
			}
			ids, err := c.termIDs(&state, term)
			if err != nil {
				return nil, 0, err
			}
			results = intersectIDs(results, ids)
			if len(results) == 0 {
				return nil, 0, nil
			}
		}
	}

	for _, term := range negative {
		ids, err := c.termIDs(&state, term)
		if err != nil {
			return nil, 0, err
		}
		results = excludeIDs(results, ids)
	}

	applySearchOrder(results, &state)
	total := len(results)
	return searchPageSlice(results, page, limit), total, nil
}

func applyOrderForInitial(ids []int, state *searchState) {
	applySearchOrder(ids, state)
}
