package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"time"
)

// hitomi の search 正典（searchlib.js）から: B=16、max_node_size=464、
// key = sha256(term) の先頭 4 bytes、node = keys / datas(offset,length) / subnode_addresses(B+1)。
// field 'galleries' は galleriesindex、field 'global'（tagindex）は suggestions 用
const btreeOrder = 16
const maxNodeSize = 464
const maxKeySize = 32

type bnode struct {
	keys  [][]byte
	datas [][2]int64
	subs  []uint64
}

// hashTerm は hitomi の hash_term と同じ（小文字化した term の sha256 の先頭 4 bytes）
func hashTerm(term string) []byte {
	sum := sha256.Sum256([]byte(strings.ToLower(term)))
	return sum[:4]
}

func compareKey(a, b []byte) int {
	top := len(a)
	if len(b) < top {
		top = len(b)
	}
	for i := 0; i < top; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// locateKey は hitomi の locate_key と同じ（first key >= search key。found は一致）
func locateKey(key []byte, node *bnode) (bool, int) {
	cmp := -1
	for i := 0; i < len(node.keys); i++ {
		cmp = compareKey(key, node.keys[i])
		if cmp <= 0 {
			return cmp == 0, i
		}
	}
	return false, len(node.keys)
}

func isLeaf(node *bnode) bool {
	for _, addr := range node.subs {
		if addr != 0 {
			return false
		}
	}
	return true
}

func decodeNode(data []byte) (*bnode, error) {
	if len(data) < 4 {
		return nil, errors.New("node too short")
	}
	node := &bnode{}
	pos := 0
	nKeys := int(binary.BigEndian.Uint32(data[pos:]))
	pos += 4
	for i := 0; i < nKeys; i++ {
		if pos+4 > len(data) {
			return nil, errors.New("node truncated at key size")
		}
		size := int(binary.BigEndian.Uint32(data[pos:]))
		pos += 4
		if size <= 0 || size > maxKeySize {
			return nil, fmt.Errorf("key_size %d out of range", size)
		}
		if pos+size > len(data) {
			return nil, errors.New("node truncated at key")
		}
		node.keys = append(node.keys, data[pos:pos+size])
		pos += size
	}
	if pos+4 > len(data) {
		return nil, errors.New("node truncated at datas count")
	}
	nDatas := int(binary.BigEndian.Uint32(data[pos:]))
	pos += 4
	if nDatas > btreeOrder+1 {
		return nil, fmt.Errorf("number_of_datas %d out of range", nDatas)
	}
	for i := 0; i < nDatas; i++ {
		if pos+12 > len(data) {
			return nil, errors.New("node truncated at data entry")
		}
		offset := int64(binary.BigEndian.Uint64(data[pos:]))
		length := int64(int32(binary.BigEndian.Uint32(data[pos+8:])))
		node.datas = append(node.datas, [2]int64{offset, length})
		pos += 12
	}
	for i := 0; i < btreeOrder+1; i++ {
		if pos+8 > len(data) {
			return nil, errors.New("node truncated at subnode address")
		}
		node.subs = append(node.subs, binary.BigEndian.Uint64(data[pos:]))
		pos += 8
	}
	return node, nil
}

func decodeIDs(data []byte) ([]int, error) {
	if len(data) < 4 {
		return nil, errors.New("data too short")
	}
	count := int(int32(binary.BigEndian.Uint32(data[:4])))
	if count <= 0 || count > 10000000 {
		return nil, fmt.Errorf("number_of_galleryids %d out of range", count)
	}
	if len(data) != count*4+4 {
		return nil, fmt.Errorf("data length %d != expected %d", len(data), count*4+4)
	}
	ids := []int{}
	for i := 4; i+4 <= len(data); i += 4 {
		ids = append(ids, int(int32(binary.BigEndian.Uint32(data[i:i+4]))))
	}
	return ids, nil
}

// indexDir は field を index dir に写す（hitomi の get_node_at_address と同じ）
func indexDir(field string) string {
	switch field {
	case "galleries":
		return "galleriesindex"
	case "languages":
		return "languagesindex"
	case "nozomiurl":
		return "nozomiurlindex"
	default:
		return "tagindex"
	}
}

func indexBaseURL(field string) string {
	return "https://ltn." + hitomiDomain + "/" + indexDir(field)
}

func (c *Client) indexVersion(field string, background bool) (string, error) {
	return c.ensureIndexVersion(field, false, background)
}

func (c *Client) ensureIndexVersion(field string, force bool, background bool) (string, error) {
	c.cacheMu.Lock()
	if c.versions == nil {
		c.versions = map[string]string{}
	}
	if c.versionAt == nil {
		c.versionAt = map[string]time.Time{}
	}
	if c.versionForceAt == nil {
		c.versionForceAt = map[string]time.Time{}
	}
	version := c.versions[field]
	age := time.Since(c.versionAt[field])
	forceMin := time.Duration(c.cfg.GGForceMinIntervalMS) * time.Millisecond
	if force && version != "" && forceMin > 0 && !c.versionForceAt[field].IsZero() && time.Since(c.versionForceAt[field]) < forceMin {
		c.cacheMu.Unlock()
		return version, nil
	}
	cached := version != "" && !force && age < indexVersionTTL
	c.cacheMu.Unlock()
	if cached {
		return version, nil
	}

	body, err := c.get(context.Background(), indexBaseURL(field)+"/version", [2]int64{-1, -1}, background)
	if err != nil {
		return "", err
	}
	version = strings.TrimSpace(string(body))
	if version == "" {
		return "", errors.New("empty index version")
	}

	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()
	if c.versions == nil {
		c.versions = map[string]string{}
	}
	if c.versionAt == nil {
		c.versionAt = map[string]time.Time{}
	}
	if c.versionForceAt == nil {
		c.versionForceAt = map[string]time.Time{}
	}
	c.versions[field] = version
	c.versionAt[field] = time.Now()
	if force {
		c.versionForceAt[field] = time.Now()
	}
	return version, nil
}

func (c *Client) indexGetRefetchWithFlag(field, version, url string, rng [2]int64, forceRefetch bool, background bool) ([]byte, string, bool, error) {
	body, err := c.get(context.Background(), url, rng, background)
	if err == nil {
		return body, version, false, nil
	}
	if !forceRefetch || !strings.Contains(err.Error(), "status 404") {
		return nil, version, false, err
	}
	newVersion, versionErr := c.ensureIndexVersion(field, true, background)
	if versionErr != nil {
		return nil, version, true, err
	}
	if newVersion != version {
		retryURL := strings.Replace(url, "."+version+".", "."+newVersion+".", 1)
		body, err := c.get(context.Background(), retryURL, rng, background)
		return body, newVersion, true, err
	}
	return nil, version, true, err
}

func (c *Client) indexGetRefetch(field, version, url string, rng [2]int64, forceRefetch bool, background bool) ([]byte, error) {
	body, _, _, err := c.indexGetRefetchWithFlag(field, version, url, rng, forceRefetch, background)
	return body, err
}

func (c *Client) indexTopNodeCached(field, version string, background bool) (*bnode, string, error) {
	key := field + "|" + version

	c.indexTopCacheMu.Lock()
	if entry, ok := c.indexTopCache[key]; ok && time.Since(entry.fetchedAt) < topCacheTTL {
		entry.recordAccess(time.Now())
		c.indexTopCacheMu.Unlock()
		return entry.node, version, nil
	}
	c.indexTopCacheMu.Unlock()

	body, actualVersion, refetched, err := c.indexGetRefetchWithFlag(
		field,
		version,
		indexBaseURL(field)+"/"+field+"."+version+".index",
		[2]int64{0, int64(maxNodeSize) - 1},
		true,
		background,
	)
	if err != nil {
		return nil, version, err
	}
	node, err := decodeNode(body)
	if err != nil {
		return nil, actualVersion, err
	}
	if !refetched && actualVersion == version {
		c.indexTopCacheMu.Lock()
		if c.indexTopCache == nil {
			c.indexTopCache = map[string]*indexTopCacheEntry{}
		}
		now := time.Now()
		c.putIndexTopCacheLocked(key, &indexTopCacheEntry{node: node, fetchedAt: now, lastAccess: now})
		c.indexTopCacheMu.Unlock()
	}
	return node, actualVersion, nil
}

func copyIDs(ids []int) []int {
	return append([]int(nil), ids...)
}

func (c *Client) indexNodeCached(field, version string, addr uint64, background bool) (*bnode, error) {
	key := fmt.Sprintf("%s|%s|%d", field, version, addr)

	c.indexCacheMu.Lock()
	if entry, ok := c.indexNodeCache[key]; ok && time.Since(entry.fetchedAt) < topCacheTTL {
		entry.recordAccess(time.Now())
		c.indexCacheMu.Unlock()
		return entry.node, nil
	}
	c.indexCacheMu.Unlock()

	body, err := c.indexGetRefetch(field, version, indexBaseURL(field)+"/"+field+"."+version+".index", [2]int64{int64(addr), int64(addr) + maxNodeSize - 1}, true, background)
	if err != nil {
		return nil, err
	}
	node, err := decodeNode(body)
	if err != nil {
		return nil, err
	}

	c.indexCacheMu.Lock()
	if c.indexNodeCache == nil {
		c.indexNodeCache = map[string]*indexNodeCacheEntry{}
	}
	now := time.Now()
	c.putIndexNodeCacheLocked(key, &indexNodeCacheEntry{node: node, fetchedAt: now, lastAccess: now})
	c.indexCacheMu.Unlock()
	return node, nil
}

func (c *Client) indexDataIDsCached(field, version string, offset, length int64, background bool) ([]int, error) {
	if length <= 0 || length > 100000000 {
		return nil, fmt.Errorf("data length %d out of range", length)
	}
	key := fmt.Sprintf("%s|%s|%d|%d", field, version, offset, length)

	c.indexCacheMu.Lock()
	if entry, ok := c.indexDataCache[key]; ok && time.Since(entry.fetchedAt) < topCacheTTL {
		entry.recordAccess(time.Now())
		c.indexCacheMu.Unlock()
		return copyIDs(entry.ids), nil
	}
	c.indexCacheMu.Unlock()

	body, err := c.indexGetRefetch(field, version, indexBaseURL(field)+"/"+field+"."+version+".data", [2]int64{offset, offset + length - 1}, true, background)
	if err != nil {
		return nil, err
	}
	ids, err := decodeIDs(body)
	if err != nil {
		return nil, err
	}

	c.indexCacheMu.Lock()
	if c.indexDataCache == nil {
		c.indexDataCache = map[string]*indexDataCacheEntry{}
	}
	now := time.Now()
	c.putIndexDataCacheLocked(key, &indexDataCacheEntry{ids: copyIDs(ids), fetchedAt: now, lastAccess: now})
	c.indexCacheMu.Unlock()
	return copyIDs(ids), nil
}

func (c *Client) indexNode(field, version string, addr uint64, background bool) (*bnode, error) {
	return c.indexNodeCached(field, version, addr, background)
}

func (c *Client) indexData(field, version string, offset, length int64, background bool) ([]byte, error) {
	if length <= 0 || length > 100000000 {
		return nil, fmt.Errorf("data length %d out of range", length)
	}
	url := indexBaseURL(field) + "/" + field + "." + version + ".data"
	return c.indexGetRefetch(field, version, url, [2]int64{offset, offset + length - 1}, true, background)
}

// IndexIDs は galleriesindex の 1 token（word）→ gallery IDs。hitomi の B-tree 降下と同じ
func (c *Client) IndexIDs(field, term string, background bool) ([]int, error) {
	version, err := c.indexVersion(field, background)
	if err != nil {
		return nil, err
	}
	node, actualVersion, err := c.indexTopNodeCached(field, version, background)
	if err != nil {
		return nil, err
	}
	version = actualVersion
	key := hashTerm(term)
	addr := uint64(0)
	for hop := 0; hop < 64; hop++ {
		if hop > 0 {
			node, err = c.indexNode(field, version, addr, background)
			if err != nil {
				return nil, err
			}
		}
		if len(node.keys) == 0 {
			return nil, nil
		}
		found, where := locateKey(key, node)
		if found {
			if where >= len(node.datas) {
				return nil, errors.New("data slot missing for key")
			}
			return c.indexDataIDsCached(field, version, node.datas[where][0], node.datas[where][1], background)
		}
		if isLeaf(node) {
			return nil, nil
		}
		if where >= len(node.subs) {
			return nil, errors.New("subnode index out of range")
		}
		if node.subs[where] == 0 {
			return nil, errors.New("non-root node address 0")
		}
		addr = node.subs[where]
	}
	return nil, errors.New("index descent too deep")
}

// indexTokens は検索 box の term を index の token に切る（小文字化・`_` は空白・区切り文字で切る）。
// 実測: `nee-san` は 1 つの key ではない（`nee` と `san` に切れる）。phrase は token 集合の共通部分
func indexTokens(phrase string) []string {
	normalized := strings.ToLower(strings.ReplaceAll(phrase, "_", " "))
	return strings.FieldsFunc(normalized, func(r rune) bool {
		return r <= ' ' || r == '-' || r == '/' || r == '#' || r == '\''
	})
}

// SearchTitle は token ごとの倒引を共通部分にする（hitomi 本体は単一 word の key しか検索しない）
func (c *Client) SearchTitle(phrase string, background bool) ([]int, error) {
	tokens := indexTokens(phrase)
	if len(tokens) == 0 {
		return nil, errors.New("phrase has no searchable token")
	}
	var ordered []int
	for i, token := range tokens {
		ids, err := c.IndexIDs("galleries", token, background)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, nil
		}
		if i == 0 {
			ordered = ids
			continue
		}
		set := map[int]bool{}
		for _, id := range ids {
			set[id] = true
		}
		next := []int{}
		for _, id := range ordered {
			if set[id] {
				next = append(next, id)
			}
		}
		ordered = next
		if len(ordered) == 0 {
			return nil, nil
		}
	}
	return ordered, nil
}
