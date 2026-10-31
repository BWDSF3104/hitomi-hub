package main

import (
	"database/sql"
	"strings"
)

// isNewGallery は galleries にまだ無い gallery id
func isNewGallery(db *sql.DB, galleryID int) (bool, error) {
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM galleries WHERE gallery_id = ?", galleryID).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

// savedQueryIDs は保存検索文を 1 回引く。namespace 付き（multi-term）は nozomi の term 共通部分（page 1・newest first）、
// namespace なし（phrase・単一 word）は galleriesindex の token 共通部分
func savedQueryIDs(client *Client, query, lang string) ([]int, error) {
	if strings.Contains(query, ":") {
		return client.SearchQuery(query, lang, 1)
	}
	return client.SearchTitle(query)
}

// runSavedQueries は保存検索の新着を追跡し、表示分を galleries に記録して last_run_at を更新
func runSavedQueries(db *sql.DB, client *Client, root, lang string, limit int) ([]map[string]any, error) {
	queries, err := listRows(db, "queries")
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, q := range queries {
		name := q["name"].(string)
		text := q["query"].(string)
		ids, err := savedQueryIDs(client, text, lang)
		if err != nil {
			out = append(out, map[string]any{"name": name, "query": text, "error": err.Error()})
			continue
		}
		newIDs := []int{}
		for _, id := range ids {
			new, err := isNewGallery(db, id)
			if err != nil {
				return nil, err
			}
			if !new {
				continue
			}
			if limit > 0 && len(newIDs) >= limit {
				break
			}
			if err := recordDiscovered(db, id, galleryDir(root, id)); err != nil {
				return nil, err
			}
			newIDs = append(newIDs, id)
		}
		if err := markQueryRun(db, q["id"].(int)); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"name": name, "query": text,
			"seen": len(ids), "new": newIDs, "count_new": len(newIDs)})
	}
	return out, nil
}
