package main

import (
	"context"
	"encoding/binary"
	"errors"
	"math/rand"
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

func normalizeSearchTerm(term string) string {
	return strings.ToLower(strings.ReplaceAll(term, "_", " "))
}

func isOrderTerm(term string) bool {
	if !strings.Contains(term, ":") {
		return false
	}
	return strings.HasPrefix(term, "orderby") || strings.HasPrefix(term, "sortby")
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

func (c *Client) SearchNozomiParts(area, tag, lang string) ([]int, error) {
	key := area + "|" + tag + "|" + lang

	c.searchCacheMu.Lock()
	if entry, ok := c.searchCache[key]; ok && time.Since(entry.fetchedAt) < searchCacheTTL {
		c.searchCacheMu.Unlock()
		return copyInts(entry.ids), nil
	}
	c.searchCacheMu.Unlock()

	body, err := c.get(context.Background(), "https://ltn."+hitomiDomain+nozomiPath(area, tag, lang), [2]int64{-1, -1})
	if err != nil {
		return nil, err
	}
	ids, err := decodeNozomiIDs(body)
	if err != nil {
		return nil, err
	}

	c.searchCacheMu.Lock()
	if c.searchCache == nil {
		c.searchCache = map[string]searchCacheEntry{}
	}
	c.searchCache[key] = searchCacheEntry{ids: copyInts(ids), fetchedAt: time.Now()}
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
		default:
			state.area = left
			state.tag = SanitizeTerm(right)
		}
		return c.SearchNozomiParts(state.area, state.tag, state.language)
	}
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
// colon term は nozomi。`language:` は state の language を変え、その後の nozomi fetch に効く。
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

	sort.SliceStable(positive, func(i, j int) bool {
		if strings.Contains(positive[i], ":") && !strings.Contains(positive[j], ":") {
			return true
		}
		if !strings.Contains(positive[i], ":") && strings.Contains(positive[j], ":") {
			return false
		}
		return false
	})

	var results []int
	orGroupOffset := 0
	if len(positive) > 0 {
		ids, err := c.termIDs(&state, positive[0])
		if err != nil {
			return nil, 0, err
		}
		results = ids
		applyOrderForInitial(results, &state)
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

	if len(positive) > 1 {
		for _, term := range positive[1:] {
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
