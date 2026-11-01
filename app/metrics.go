package main

import (
	"strings"
	"time"
)

type FetchMetric struct {
	Calls       int
	GateWaitMS  int64
	NetworkMS   int64
	DecodeMS    int64
	RetrySleepMS int64
}

func classifyFetchURL(rawurl string) string {
	switch {
	case strings.HasSuffix(rawurl, "/version"):
		return "version"
	case strings.Contains(rawurl, ".index"):
		return "index"
	case strings.Contains(rawurl, "nozomi"):
		return "nozomi"
	case strings.Contains(rawurl, "gg.js") || strings.Contains(rawurl, "/gg/"):
		return "gg"
	case strings.Contains(rawurl, "tn.") ||
		strings.Contains(rawurl, "webpbigtn") ||
		strings.Contains(rawurl, "avifbigtn") ||
		strings.Contains(rawurl, "webpsmalltn") ||
		strings.Contains(rawurl, "webpsmallsmalltn"):
		return "image"
	case strings.Contains(rawurl, "galleryinfo") || strings.Contains(rawurl, ".html"):
		return "gallery"
	default:
		return "data"
	}
}

func (c *Client) metricFor(kind string) *FetchMetric {
	if c.metrics == nil {
		c.metrics = map[string]*FetchMetric{}
	}
	m := c.metrics[kind]
	if m == nil {
		m = &FetchMetric{}
		c.metrics[kind] = m
	}
	return m
}

func (c *Client) recordMetric(kind string, gateWait, network, decode, retrySleep time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.metricFor(kind)
	m.Calls++
	m.GateWaitMS += int64(gateWait / time.Millisecond)
	m.NetworkMS += int64(network / time.Millisecond)
	m.DecodeMS += int64(decode / time.Millisecond)
	m.RetrySleepMS += int64(retrySleep / time.Millisecond)
}

func (c *Client) metricsSnapshot() map[string]any {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]any{}
	for kind, m := range c.metrics {
		out[kind] = map[string]int64{
			"calls":         int64(m.Calls),
			"gate_wait_ms":  m.GateWaitMS,
			"network_ms":    m.NetworkMS,
			"decode_ms":     m.DecodeMS,
			"retry_sleep_ms": m.RetrySleepMS,
		}
	}
	return out
}
