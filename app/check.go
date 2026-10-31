package main

import (
	"database/sql"
)

// isNewGallery は galleries にまだ無い gallery id
func isNewGallery(db *sql.DB, galleryID int) (bool, error) {
	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM galleries WHERE gallery_id = ?", galleryID).Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

// savedQueryIDs は保存検索文を hitomi の results.js と同じ routing で 1 回引く。plain term は galleriesindex exact hash、colon term は nozomi。
func savedQueryIDs(client *Client, query, lang string) ([]int, error) {
	ids, _, err := client.SearchHTML(query, lang, 1, -1)
	if err != nil {
		return nil, err
	}
	return ids, nil
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
