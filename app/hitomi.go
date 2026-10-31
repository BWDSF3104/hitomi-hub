package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// hitomi の data 正典（ltn）。公式 API は無い。正典ドメインは ltn の common.js（domain2）から
const hitomiDomain = "gold-usergeneratedcontent.net"
const ggBucket = "1793469601/"
const galleriesPerPage = 25

type HitomiConfig struct {
	MinIntervalMS int    `json:"min_interval_ms"`
	MaxRetries    int    `json:"max_retries"`
	RetryBackoffMS int   `json:"retry_backoff_ms"`
	UserAgent     string `json:"user_agent"`
	ImageFormat   string `json:"image_format"`
}

type Client struct {
	http   *http.Client
	cfg    HitomiConfig
	last   time.Time
	fetch  func(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error)
}

func defaultHitomiConfig() HitomiConfig {
	return HitomiConfig{MinIntervalMS: 1000, MaxRetries: 3, RetryBackoffMS: 2000,
		UserAgent: "hitomi-hub/0.1 (local hub client)", ImageFormat: "webp"}
}

func (c *Client) gate() {
	if c.cfg.MinIntervalMS <= 0 {
		return
	}
	if wait := time.Duration(c.cfg.MinIntervalMS) * time.Millisecond; time.Since(c.last) < wait {
		time.Sleep(wait - time.Since(c.last))
	}
	c.last = time.Now()
}

func (c *Client) get(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
	if c.fetch != nil {
		c.gate()
		return c.fetch(ctx, rawurl, rng)
	}
	return c.fetchRemote(ctx, rawurl, rng)
}

func (c *Client) fetchRemote(ctx context.Context, rawurl string, rng [2]int64) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.MaxRetries; attempt++ {
		c.gate()
		req, err := http.NewRequestWithContext(ctx, "GET", rawurl, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("user-agent", c.cfg.UserAgent)
		req.Header.Set("referer", "https://hitomi.la/")
		req.Header.Set("accept", "*/*")
		if rng[0] >= 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", rng[0], rng[1]))
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
		} else {
			body, readErr := readAll(resp)
			resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
			} else if resp.StatusCode == 200 || resp.StatusCode == 206 {
				return body, nil
			}
			lastErr = fmt.Errorf("status %d for %s", resp.StatusCode, rawurl)
			if resp.StatusCode == 429 {
				time.Sleep(time.Duration(c.cfg.RetryBackoffMS*(attempt+1)) * time.Millisecond)
				continue
			}
			if resp.StatusCode >= 500 {
				time.Sleep(time.Duration(c.cfg.RetryBackoffMS*(attempt+1)) * time.Millisecond)
				continue
			}
			return nil, lastErr
		}
		time.Sleep(time.Duration(c.cfg.RetryBackoffMS*(attempt+1)) * time.Millisecond)
	}
	return nil, lastErr
}

func readAll(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}

// ggValue は gg.s(hash) と同じ（末尾 3 hex を反転して 10 進）
func ggValue(hash string) int {
	if len(hash) < 3 {
		return 0
	}
	v, err := strconv.ParseInt(string(hash[len(hash)-1])+hash[len(hash)-3:len(hash)-1], 16, 64)
	if err != nil {
		return 0
	}
	return int(v)
}

func dirLetter(dir string) string {
	if dir == "avif" {
		return "a"
	}
	return "w"
}

// ImageURL は gg.js の url_from_hash + subdomain_from_url（base なし・webp）と同じ
func ImageURL(hash, dir, ext string) string {
	g := ggValue(hash)
	if ext == "" {
		ext = dir
	}
	sub := dirLetter(dir) + strconv.Itoa(1+ggM(g))
	return "https://" + sub + "." + hitomiDomain + "/" + ggBucket + strconv.Itoa(g) + "/" + hash + "." + ext
}

// StorageURL は shard 上の保存形式で URL を組む。shard には webp / avif ののみ格納（実測:
// 実測で webp 200・同じ hash の jpg 404）。galleryinfo の `name` の ext は元の形式なので URL に使わない。
func StorageURL(hash, format string) string {
	if format == "" {
		format = "webp"
	}
	return ImageURL(hash, format, format)
}

// SanitizeTerm は hitomi の sanitize と同じ形（小文字化・空白を _・区切り文字を保持）
func SanitizeTerm(term string) string {
	return strings.ToLower(strings.ReplaceAll(term, " ", "_"))
}

type FileEntry struct {
	Hash   string `json:"hash"`
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type GalleryInfo struct {
	ID            int         `json:"id"`
	Title         string      `json:"title"`
	JapaneseTitle string      `json:"japanese_title"`
	Language      string      `json:"language"`
	Type          string      `json:"type"`
	Tags          []string    `json:"tags"`
	Artists       []string    `json:"artists"`
	Groups        []string    `json:"groups"`
	Characters    []string    `json:"characters"`
	URL           string      `json:"galleryurl"`
	Files         []FileEntry `json:"files"`
}

type rawGallery struct {
	ID            string              `json:"id"`
	Title         string              `json:"title"`
	JapaneseTitle string              `json:"japanese_title"`
	Language      string              `json:"language"`
	Type          string              `json:"type"`
	Tags          []map[string]string `json:"tags"`
	Artists       []map[string]string `json:"artists"`
	Groups        []map[string]string `json:"groups"`
	Characters    []map[string]string `json:"characters"`
	GalleryURL    string              `json:"galleryurl"`
	Files         []FileEntry         `json:"files"`
}

func names(entries []map[string]string, key string) []string {
	out := []string{}
	for _, entry := range entries {
		if entry == nil || entry[key] == "" {
			continue
		}
		if entry["female"] != "" {
			out = append(out, "female:"+entry[key])
		} else if entry["male"] != "" {
			out = append(out, "male:"+entry[key])
		} else {
			out = append(out, entry[key])
		}
	}
	return out
}

func (c *Client) Gallery(id int) (*GalleryInfo, error) {
	body, err := c.get(context.Background(), "https://ltn."+hitomiDomain+"/galleries/"+strconv.Itoa(id)+".js", [2]int64{-1, -1})
	if err != nil {
		return nil, err
	}
	text := string(body)
	idx := strings.Index(text, "var galleryinfo = ")
	if idx < 0 {
		return nil, fmt.Errorf("galleryinfo not found for %d", id)
	}
	raw := &rawGallery{}
	if err := json.Unmarshal([]byte(text[idx+len("var galleryinfo = "):]), raw); err != nil {
		return nil, err
	}
	galleryID, err := strconv.Atoi(raw.ID)
	if err != nil {
		return nil, fmt.Errorf("gallery id %q: %s", raw.ID, err)
	}
	return &GalleryInfo{
		ID: galleryID, Title: raw.Title, JapaneseTitle: raw.JapaneseTitle, Language: raw.Language,
		Type: raw.Type, URL: raw.GalleryURL, Files: raw.Files,
		Tags: names(raw.Tags, "tag"), Artists: names(raw.Artists, "artist"),
		Groups: names(raw.Groups, "group"), Characters: names(raw.Characters, "character"),
	}, nil
}

// ExtractGalleryID は hitomi の GetGalleryId と同じ（URL の <digits>.html から id）
func ExtractGalleryID(raw string) (int, error) {
	m := galleryIDPattern.FindString(raw)
	if m == "" {
		return 0, errors.New("no gallery id in URL")
	}
	return strconv.Atoi(strings.TrimSuffix(m, ".html"))
}

var galleryIDPattern = regexp.MustCompile(`[0-9]+\.html`)

// nozomiPath は hitomi の get_galleryids_for_query と同じ分区規則
// female:/male: は area=tag（tag にはクエリ全体）、その他は area=namespace・tag=右辺、
// language: は lang に反映。area が空の plain query（タイトル検索）は index が必要で未実装。
func nozomiPath(term, lang string) (string, string, error) {
	area := ""
	tag := SanitizeTerm(term)
	if i := strings.Index(term, ":"); i >= 0 {
		left, right := strings.ToLower(term[:i]), SanitizeTerm(term[i+1:])
		switch left {
		case "female", "male":
			area = "tag"
			tag = SanitizeTerm(term)
		case "language":
			lang = strings.ToLower(right)
		default:
			area = left
			tag = right
		}
	}
	if tag == "" {
		return "", "", errors.New("term has no tag part (plain title search needs the index)")
	}
	path := "/n/"
	if area != "" {
		path += url.PathEscape(area) + "/"
	}
	return path + url.PathEscape(tag+"-"+lang) + ".nozomi", lang, nil
}

// Search は nozomi（big-endian int32 の gallery ID 配列）を読む。page は 1 始まり
func (c *Client) Search(term, lang string, page int) ([]int, error) {
	if page < 1 {
		page = 1
	}
	path, lang, err := nozomiPath(term, lang)
	if err != nil {
		return nil, err
	}
	full := "https://ltn." + hitomiDomain + path
	start := int64((page-1)*galleriesPerPage*4)
	body, err := c.get(context.Background(), full, [2]int64{start, start + int64(galleriesPerPage*4) - 1})
	if err != nil {
		return nil, err
	}
	ids := []int{}
	for i := 0; i+4 <= len(body); i += 4 {
		ids = append(ids, int(binary.BigEndian.Uint32(body[i:i+4])))
	}
	return ids, nil
}
