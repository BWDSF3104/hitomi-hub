package main

import (
	"errors"
	"net/http"
	"strings"
)

// jump-to-first-status-page（decision 2026-11-05-jump-first-status-page-design-approved）。
// status mask bit は badge 4状態（decision 2026-11-05-search-result-state-badges-approved）の
// 名称と 1:1 で対応する: viewed / read（fully_read）/ saved / favorite。既定 mask は 4状態の
// いずれか（statusMaskDefault）。単一/複数選択可。

const (
	statusBitViewed   = 1
	statusBitRead     = 2
	statusBitSaved    = 4
	statusBitFavorite = 8
	statusMaskDefault = statusBitViewed | statusBitRead | statusBitSaved | statusBitFavorite
)

// errRandomOrderNotSupported: random order は「最初のページ」が呼出ごとに変わるため無効
// （decision 2026-11-05-jump-first-status-page-random-order-disabled）。UI は button disabled、
// CLI/API はこの error。
var errRandomOrderNotSupported = errors.New("random order is not supported")

// parseStatusMask は comma 区切りの status 名（viewed|read|saved|favorite）を mask へ。
// 空文字は既定（4状態のいずれか）。CLI `--status`・API `status` 同一解析。
func parseStatusMask(raw string) (int, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return statusMaskDefault, nil
	}
	mask := 0
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch part {
		case "viewed":
			mask |= statusBitViewed
		case "read":
			mask |= statusBitRead
		case "saved":
			mask |= statusBitSaved
		case "favorite":
			mask |= statusBitFavorite
		default:
			return 0, errors.New("status must be viewed|read|saved|favorite (comma separated)")
		}
	}
	if mask == 0 {
		return 0, errors.New("status must be viewed|read|saved|favorite (comma separated)")
	}
	return mask, nil
}

// firstStatusResult は返す値 page・rank・hit gallery_id・hit flags・total（設計確定 2026-11-05）。
// hit なしは Hit=false（JSON へ hit null）。
type firstStatusResult struct {
	Hit   bool
	Page  int
	Rank  int
	ID    int
	Flags map[string]any
	Total int
}

// firstStatusJSON は API と CLI で同一 JSON 形状。hit なしは hit null（total は dataset total）。
func firstStatusJSON(query, lang string, mask int, res *firstStatusResult) map[string]any {
	out := map[string]any{
		"query":       query,
		"lang":        lang,
		"status_mask": mask,
		"total":       res.Total,
		"hit":         nil,
	}
	if res.Hit {
		out["hit"] = map[string]any{
			"page":       res.Page,
			"rank":       res.Rank,
			"gallery_id": res.ID,
			"flags":      res.Flags,
		}
	}
	return out
}

func statusFlagsMap(mask int) map[string]any {
	return map[string]any{
		"viewed":   boolToInt(mask&statusBitViewed != 0),
		"read":     boolToInt(mask&statusBitRead != 0),
		"saved":    boolToInt(mask&statusBitSaved != 0),
		"favorite": boolToInt(mask&statusBitFavorite != 0),
	}
}

// firstStatus は現在の検索条件の full ordered IDs を searchAllIDs（既存 searchCache 共有・追加
// fetch なし）から取り、rank 0 順に walk して最初の一 hit を返す。candidate set は 1 query
// （read_state flags）。saved は event 列 saved_at（decision 2026-11-06-saved-as-event-record-approved
// ・完成後削除で false へ戻さない）。pre-filter を event 源へ切り替えたことで candidate（record）と
// flag（event）の分岐が解消される（旧 savedStrict の disk 逐再計算と files EXISTS record 源は廃止）。
// read-only（DB 書込みは target page 描画時の既存 recordDiscoveredBatch のみで、jump 計算では書かない）。
// hit なしは Hit=false・total=dataset total。random order は error。
func (v *viewer) firstStatus(query, lang string, mask int) (*firstStatusResult, error) {
	terms := []string{}
	for _, rawTerm := range strings.Fields(query) {
		terms = append(terms, normalizeSearchTerm(rawTerm))
	}
	state, _, err := buildSearchState(terms, lang, false)
	if err != nil {
		return nil, err
	}
	if state.orderbydir == "random" {
		return nil, errRandomOrderNotSupported
	}
	ids, err := v.client.searchAllIDs(query, lang, false)
	if err != nil {
		return nil, err
	}
	res := &firstStatusResult{Total: len(ids)}
	if len(ids) == 0 {
		return res, nil
	}

	// candidate set を 1 query で構築: read_state の flags（mask）。saved は event 列 saved_at（event 源）。
	// flag は mask bit membership として 0/1 で数え、Go 側で bit へ写す。
	rows, err := v.db.Query(`SELECT gallery_id,
		(? & 1) > 0 AND COALESCE(viewed, 0) > 0,
		(? & 2) > 0 AND COALESCE(fully_read, 0) > 0,
		(? & 4) > 0 AND saved_at IS NOT NULL,
		(? & 8) > 0 AND COALESCE(favorite, 0) > 0
		FROM read_state
		WHERE ((? & 1) > 0 AND COALESCE(viewed, 0) > 0)
		   OR ((? & 2) > 0 AND COALESCE(fully_read, 0) > 0)
		   OR ((? & 4) > 0 AND saved_at IS NOT NULL)
		   OR ((? & 8) > 0 AND COALESCE(favorite, 0) > 0)`, mask, mask, mask, mask, mask, mask, mask, mask)
	if err != nil {
		return nil, err
	}
	candidates := map[int]int{}
	for rows.Next() {
		var id, viewed, read, savedPre, favorite int
		if err := rows.Scan(&id, &viewed, &read, &savedPre, &favorite); err != nil {
			rows.Close()
			return nil, err
		}
		flags := viewed*statusBitViewed | read*statusBitRead | savedPre*statusBitSaved | favorite*statusBitFavorite
		candidates[id] = flags
	}
	rows.Close()

	for rank, id := range ids {
		flags := candidates[id]
		if flags == 0 {
			continue
		}
		res.Hit = true
		res.Page = rank/galleriesPerPage + 1
		res.Rank = rank
		res.ID = id
		res.Flags = statusFlagsMap(flags)
		return res, nil
	}
	return res, nil
}

// searchFirstStatus は GET /api/search-first-status（foreground・read-only）。
func (v *viewer) searchFirstStatus(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	lang := r.URL.Query().Get("lang")
	mask, err := parseStatusMask(r.URL.Query().Get("status"))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := v.firstStatus(query, lang, mask)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, errRandomOrderNotSupported) {
			status = http.StatusBadRequest
		}
		writeJSONError(w, status, err.Error())
		return
	}
	jsonOut(w, firstStatusJSON(query, lang, mask, res))
}
