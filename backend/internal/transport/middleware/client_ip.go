package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

func rateLimitClientIP(r *http.Request, trustedProxy string) string {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peer = r.RemoteAddr
	}
	// Traefik appends its client's address. Only the final value is trusted,
	// so a caller-supplied prefix cannot choose or evade the quota key.
	forwarded := r.Header.Values("X-Forwarded-For")
	if trustedProxy == "" || len(forwarded) == 0 {
		return peer
	}
	last := forwarded[len(forwarded)-1]
	if comma := strings.LastIndexByte(last, ','); comma >= 0 {
		last = last[comma+1:]
	}
	clientIP, err := netip.ParseAddr(strings.TrimSpace(last))
	if err != nil {
		return peer
	}
	peerIP, err := netip.ParseAddr(peer)
	if err != nil {
		return peer
	}
	if proxyIP, err := netip.ParseAddr(trustedProxy); err == nil {
		if proxyIP.Unmap() == peerIP.Unmap() {
			return clientIP.Unmap().String()
		}
		return peer
	}
	// Resolve the explicitly configured proxy name, rather than trusting a whole
	// private subnet. Resolve per request so a recreated Docker proxy can change IP.
	ctx, cancel := context.WithTimeout(r.Context(), 250*time.Millisecond)
	defer cancel()
	proxies, err := net.DefaultResolver.LookupNetIP(ctx, "ip", trustedProxy)
	if err != nil {
		slog.Warn("cannot resolve trusted rate limit proxy; using peer IP", "error", err)
		return peer
	}
	for _, proxyIP := range proxies {
		if proxyIP.Unmap() == peerIP.Unmap() {
			return clientIP.Unmap().String()
		}
	}
	return peer
}
