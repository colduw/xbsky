package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"main/internal/types"
)

const (
	MaxReadLimit = 10 * (1024 * 1024)
)

var (
	IsBlueskyDead atomic.Bool

	SDialer = &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   SDial,
	}

	TimeoutClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           SDialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       time.Minute,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
	}
)

func ResolveHandleAPI(ctx context.Context, handle string) (string, bool) {
	apiURL := "https://public.api.bsky.app/xrpc/com.atproto.identity.resolveHandle?handle=" + handle
	if IsBlueskyDead.Load() {
		apiURL = "https://api.bsky.app/xrpc/com.atproto.identity.resolveHandle?handle=" + handle
	}

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		return handle, false
	}

	resp, respErr := TimeoutClient.Do(req)
	if respErr != nil {
		return handle, false
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return handle, false
	}

	var uDID types.APIDID
	if decodeErr := json.NewDecoder(resp.Body).Decode(&uDID); decodeErr != nil {
		return handle, false
	}

	if !strings.HasPrefix(uDID.DID, "did:") {
		return handle, false
	}

	return uDID.DID, true
}

func ResolveHandleDNS(ctx context.Context, handle string) (string, bool) {
	records, lookupErr := net.DefaultResolver.LookupTXT(ctx, "_atproto."+handle)
	if lookupErr != nil {
		return handle, false
	}

	if len(records) > 0 {
		if didfound, ok := strings.CutPrefix(records[0], "did="); ok {
			return didfound, true
		}
	}

	return handle, false
}

func ResolveHandleHTTP(ctx context.Context, handle string) (string, bool) {
	atURL := fmt.Sprintf("https://%s/.well-known/atproto-did", handle)

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, atURL, http.NoBody)
	if reqErr != nil {
		return handle, false
	}

	resp, respErr := TimeoutClient.Do(req)
	if respErr != nil {
		return handle, false
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return handle, false
	}

	// https://github.com/did-method-plc/did-method-plc?tab=readme-ov-file#identifier-syntax
	body, bodyErr := io.ReadAll(io.LimitReader(resp.Body, 32))
	if bodyErr != nil {
		return handle, false
	}

	responseBody := string(body)

	if !strings.HasPrefix(responseBody, "did:") {
		return handle, false
	}

	return responseBody, true
}

// https://atproto.com/specs/handle#handle-resolution
func ResolveHandle(ctx context.Context, handle string) string {
	// Try using the API first
	if did, ok := ResolveHandleAPI(ctx, handle); ok {
		return did
	}

	// Try using DNS
	if did, ok := ResolveHandleDNS(ctx, handle); ok {
		return did
	}

	// Try using .well-known
	if did, ok := ResolveHandleHTTP(ctx, handle); ok {
		return did
	}

	// Failed to find DID, use the handle we got
	return handle
}

func ResolvePLC(ctx context.Context, did string) types.PLCDirectory {
	var didURL string

	// https://atproto.com/specs/did#blessed-did-methods
	if strings.HasPrefix(did, "did:plc:") {
		didURL = "https://plc.directory/" + did
	} else if didweb, ok := strings.CutPrefix(did, "did:web:"); ok {
		didURL = fmt.Sprintf("https://%s/.well-known/did.json", didweb)
	} else {
		return types.PLCDirectory{}
	}

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, didURL, http.NoBody)
	if reqErr != nil {
		return types.PLCDirectory{}
	}

	resp, respErr := TimeoutClient.Do(req)
	if respErr != nil {
		return types.PLCDirectory{}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return types.PLCDirectory{}
	}

	var plc types.PLCDirectory

	if decodeErr := json.NewDecoder(io.LimitReader(resp.Body, MaxReadLimit)).Decode(&plc); decodeErr != nil {
		return types.PLCDirectory{}
	}

	return plc
}
