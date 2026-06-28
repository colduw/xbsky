package helpers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func ToNotation(number int64) string {
	switch {
	case number >= 1e9:
		return strconv.FormatFloat(float64(number)/1e9, 'f', 1, 64) + "B"
	case number >= 1e6:
		return strconv.FormatFloat(float64(number)/1e6, 'f', 1, 64) + "M"
	case number >= 1e3:
		return strconv.FormatFloat(float64(number)/1e3, 'f', 1, 64) + "K"
	default:
		return strconv.FormatInt(number, 10)
	}
}

func NL2BR(in string) string {
	// This is escaped, but it somehow works.
	// I don't know, and I don't wanna know.
	return strings.ReplaceAll(in, "\n", "<br>")
}

// Check if bluesky is having issues (https://public.api.bsky.app/xrpc/_health)
// If this returns a non 200, it is most likely down (probably due to their ai slop usage)
// In that case, rewrite it to use the "private" api, which is the same, just w/o caching
// Not a guaranteed fix, since it works 50/50, but still better than the guaranteed down public api
func BlueskyHealthCheck() {
	ticker := time.NewTicker(10 * time.Minute)

	for range ticker.C {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://public.api.bsky.app/xrpc/_health", http.NoBody)
		if err != nil {
			IsBlueskyDead.Store(true)
			continue
		}

		resp, err := TimeoutClient.Do(req)
		if err != nil {
			IsBlueskyDead.Store(true)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			IsBlueskyDead.Store(true)
			continue
		}

		resp.Body.Close()
		IsBlueskyDead.Store(false)
	}
}

// https://www.agwa.name/blog/post/preventing_server_side_request_forgery_in_golang
func SDial(network, addr string, conn syscall.RawConn) error {
	if network != "tcp4" && network != "tcp6" {
		return errors.New("bad network type")
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return errors.New("bad address")
	}

	if port != "80" && port != "443" {
		return errors.New("bad port")
	}

	// https://stackoverflow.com/a/50825191 && https://stackoverflow.com/a/67526079
	ips, err := net.DefaultResolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return errors.New("failed host lookup")
	}

	for _, v := range ips {
		if !v.IP.IsGlobalUnicast() ||
			v.IP.IsLoopback() ||
			v.IP.IsLinkLocalUnicast() ||
			v.IP.IsLinkLocalMulticast() ||
			v.IP.IsPrivate() ||
			v.IP.IsUnspecified() {
			return errors.New("invalid host")
		}
	}

	return nil
}

func LoadEnv() error {
	envFile, err := os.Open(".env")
	if err != nil {
		return fmt.Errorf("failed to open .env file: %w", err)
	}
	defer envFile.Close() //nolint:errcheck // should not fail under normal circumstances

	scanner := bufio.NewScanner(envFile)
	for scanner.Scan() {
		envLine := scanner.Text()
		if strings.HasPrefix(envLine, "#") {
			continue
		}

		envName, envValue, ok := strings.Cut(envLine, "=")
		if !ok {
			continue
		}

		if err := os.Setenv(envName, envValue); err != nil {
			return fmt.Errorf("failed to set environment variable: %w", err)
		}
	}

	return nil
}
