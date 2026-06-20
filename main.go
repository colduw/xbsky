package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

type (
	userProfile struct {
		Handle         string `json:"handle"`
		DisplayName    string `json:"displayName"`
		Avatar         string `json:"avatar"`
		Description    string `json:"description"`
		CreatedAt      string `json:"createdAt"`
		FollowersCount int64  `json:"followersCount"`
		FollowsCount   int64  `json:"followsCount"`
		PostsCount     int64  `json:"postsCount"`
		Associated     struct {
			Labeler bool `json:"labeler"`
		} `json:"associated"`
	}

	apiDID struct {
		DID string `json:"did"`
	}

	apiThread struct {
		Thread struct {
			// This is the main post
			Post apiPost `json:"post"`
			// Parent, if this is a reply to an already existing post
			// Also a pointer, so if there is no reply, this is nil
			Parent *struct {
				Post apiPost `json:"post"`
			} `json:"parent"`
		} `json:"thread"`
	}

	apiFeed struct {
		View struct {
			DisplayName string    `json:"displayName"`
			Description string    `json:"description"`
			Avatar      string    `json:"avatar"`
			IndexedAt   string    `json:"indexedAt"`
			Creator     apiAuthor `json:"creator"`
			LikeCount   int64     `json:"likeCount"`
		} `json:"view"`

		IsOnline bool `json:"isOnline"`
		IsValid  bool `json:"isValid"`
	}

	apiList struct {
		List struct {
			Name        string    `json:"name"`
			Purpose     string    `json:"purpose"`
			Avatar      string    `json:"avatar"`
			Description string    `json:"description"`
			IndexedAt   string    `json:"indexedAt"`
			Creator     apiAuthor `json:"creator"`
			ItemCount   int64     `json:"listItemCount"`
		} `json:"list"`
	}

	apiPack struct {
		StarterPack struct {
			Record struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				CreatedAt   string `json:"createdAt"`
			} `json:"record"`

			Creator apiAuthor `json:"creator"`
		} `json:"starterPack"`
	}

	apiImages []struct {
		FullSize    string         `json:"fullsize"`
		Alt         string         `json:"alt"`
		AspectRatio apiAspectRatio `json:"aspectRatio"`
	}

	apiAuthor struct {
		DID         string `json:"did"`
		Handle      string `json:"handle"`
		DisplayName string `json:"displayName"`
		Avatar      string `json:"avatar"`
	}

	apiExternal struct {
		URI         string `json:"uri"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Thumb       string `json:"thumb"`
	}

	apiPost struct {
		URI string `json:"uri"`

		Author apiAuthor `json:"author"`

		// Text of the post
		Record struct {
			Text      string `json:"text"`
			CreatedAt string `json:"createdAt"`

			Facets []struct {
				Features []struct {
					Type string `json:"$type"`
					URI  string `json:"uri"`
					Tag  string `json:"tag"`
					DID  string `json:"did"`
				} `json:"features"`

				Index struct {
					ByteStart int64 `json:"byteStart"`
					ByteEnd   int64 `json:"byteEnd"`
				} `json:"index"`
			} `json:"facets"`
		} `json:"record"`

		// Embeds of stuff, if any.
		Embed struct {
			Type string `json:"$type"`

			// If this is a quote, and if there are embeds,
			// they'll be here
			Media mediaData `json:"media"`

			External apiExternal `json:"external"`

			// This is a text quote
			Record struct {
				Type string `json:"$type"`

				// This is for starter packs (it contains the quotee's id)
				URI string `json:"uri"`

				// This is a quote with media
				Record struct {
					Value struct {
						Text   string `json:"text"`
						Facets []struct {
							Features []struct {
								Type string `json:"$type"`
								URI  string `json:"uri"`
								Tag  string `json:"tag"`
								DID  string `json:"did"`
							} `json:"features"`

							Index struct {
								ByteStart int64 `json:"byteStart"`
								ByteEnd   int64 `json:"byteEnd"`
							} `json:"index"`
						} `json:"facets"`
					} `json:"value"`

					Author apiAuthor `json:"author"`

					URI string `json:"uri"`

					// This is for starter packs
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"record"`

				Value struct {
					Text string `json:"text"`

					Facets []struct {
						Features []struct {
							Type string `json:"$type"`
							URI  string `json:"uri"`
							Tag  string `json:"tag"`
							DID  string `json:"did"`
						} `json:"features"`

						Index struct {
							ByteStart int64 `json:"byteStart"`
							ByteEnd   int64 `json:"byteEnd"`
						} `json:"index"`
					} `json:"facets"`
				} `json:"value"`

				Author apiAuthor `json:"author"`

				Embeds []struct {
					mediaData
					Media mediaData `json:"media"`

					Record struct {
						Type string `json:"$type"`

						// This is for starter packs
						URI string `json:"uri"`

						// This is for starter packs
						Record struct {
							Description string `json:"description"`
							Name        string `json:"name"`
						} `json:"record"`

						// This is for feeds
						DisplayName string `json:"displayName"`

						// This is for lists
						Purpose string `json:"purpose"`

						// Found in lists, starter packs, feeds
						Name        string    `json:"name"`
						Avatar      string    `json:"avatar"`
						Description string    `json:"description"`
						Creator     apiAuthor `json:"creator"`
					} `json:"record"`
				} `json:"embeds"`

				// This is for feeds
				DisplayName string `json:"displayName"`

				// This is for lists
				Purpose string `json:"purpose"`

				// Found in lists, starter packs, feeds
				Name        string    `json:"name"`
				Avatar      string    `json:"avatar"`
				Description string    `json:"description"`
				Creator     apiAuthor `json:"creator"`
			} `json:"record"`

			Images apiImages `json:"images"`

			// Gallery (10+ images)
			// Why is it called "items"? Who knows.
			Items apiImages `json:"items"`

			CID         string         `json:"cid"`
			Thumbnail   string         `json:"thumbnail"`
			AspectRatio apiAspectRatio `json:"aspectRatio"`
		} `json:"embed"`

		ReplyCount  int64 `json:"replyCount"`
		RepostCount int64 `json:"repostCount"`
		LikeCount   int64 `json:"likeCount"`
		QuoteCount  int64 `json:"quoteCount"`
	}

	mediaData struct {
		Type string `json:"$type"`

		Images apiImages `json:"images"`

		Items apiImages `json:"items"`

		External apiExternal `json:"external"`

		CID         string         `json:"cid"`
		Thumbnail   string         `json:"thumbnail"`
		AspectRatio apiAspectRatio `json:"aspectRatio"`
	}

	apiAspectRatio struct {
		Width  int64 `json:"width"`
		Height int64 `json:"height"`
	}

	oEmbed struct {
		Version      string `json:"version"`
		Type         string `json:"type"`
		ProviderName string `json:"provider_name"`
		ProviderURL  string `json:"provider_url"`
		AuthorName   string `json:"author_name"`
	}

	richActivityEncoded struct {
		Type     string `json:"t"`
		Handle   string `json:"h"`
		PostID   string `json:"p"`
		PhotoCut string `json:"c"`
	}

	richActivity struct {
		ID               string                  `json:"id"`
		URL              string                  `json:"url"`
		URI              string                  `json:"uri"`
		CreatedAt        string                  `json:"created_at"`
		Language         string                  `json:"language"` // "en"
		Content          string                  `json:"content"`
		SpoilerText      string                  `json:"spoiler_text"` // Title
		Visibility       string                  `json:"visibility"`   // "public"
		Application      richActivityApplication `json:"application"`
		Account          richActivityAccount     `json:"account"`
		MediaAttachments []richActivityMedia     `json:"media_attachments"`
	}

	richActivityApplication struct {
		Name    string `json:"name"`
		Website string `json:"website"`
	}

	richActivityMedia struct {
		ID          string `json:"id"`
		Type        string `json:"type"`
		URL         string `json:"url"`
		Preview     string `json:"preview_url"`
		Description string `json:"description"`
	}

	richActivityAccount struct {
		ID           string `json:"id"`
		DisplayName  string `json:"display_name"`
		UserName     string `json:"username"`
		Acct         string `json:"acct"`
		URL          string `json:"url"`
		URI          string `json:"uri"`
		Avatar       string `json:"avatar"`
		AvatarStatic string `json:"avatar_static"`
	}

	// https://atproto.com/specs/did#did-documents
	plcDirectory struct {
		AKA     []string `json:"alsoKnownAs"`
		Service []struct {
			ID       string `json:"id"`
			Type     string `json:"type"`
			Endpoint string `json:"serviceEndpoint"`
		} `json:"service"`
	}

	// To reduce redundancy in the template
	ownData struct {
		Type string `json:"type"`

		Author apiAuthor `json:"author"`

		Record struct {
			Text      string `json:"text"`
			CreatedAt string `json:"createdAt"`

			Facets []struct {
				Features []struct {
					Type string `json:"$type"`
					URI  string `json:"uri"`
					Tag  string `json:"tag"`
					DID  string `json:"did"`
				} `json:"features"`

				Index struct {
					ByteStart int64 `json:"byteStart"`
					ByteEnd   int64 `json:"byteEnd"`
				} `json:"index"`
			} `json:"facets"`
		} `json:"record"`

		Images apiImages `json:"images"`

		External apiExternal `json:"external"`

		PDS         string `json:"pds"`
		VideoCID    string `json:"videoCID"`
		VideoDID    string `json:"videoDID"`
		VideoHelper string `json:"videoURI"`

		Description string `json:"description"`
		StatsForTG  string `json:"statsForTG"`

		Thumbnail   string         `json:"thumbnail"`
		AspectRatio apiAspectRatio `json:"aspectRatio"`

		ReplyCount  int64 `json:"replyCount"`
		RepostCount int64 `json:"repostCount"`
		LikeCount   int64 `json:"likeCount"`
		QuoteCount  int64 `json:"quoteCount"`

		IsVideo bool `json:"isVideo"`
		IsGif   bool `json:"isGif"`

		OriginalPostID string `json:"originalPostID"`

		CommonEmbeds struct {
			Purpose     string    `json:"purpose"`
			Name        string    `json:"name"`
			Avatar      string    `json:"avatar"`
			Description string    `json:"description"`
			Creator     apiAuthor `json:"creator"`
		} `json:"commonEmbeds"`
	}

	SortedAPIResponse struct {
		OriginalData apiThread `json:"originalData"`
		ParsedData   ownData   `json:"parsedData"`
	}
)

const (
	maxAuthorLen = 256
	ellipsisLen  = 3
	maxReadLimit = 10 * (1024 * 1024)

	bskyEmbedImages    = "app.bsky.embed.images#view"
	galleryImages      = "app.bsky.embed.gallery#view"
	bskyEmbedExternal  = "app.bsky.embed.external#view"
	bskyEmbedVideo     = "app.bsky.embed.video#view"
	bskyEmbedQuote     = "app.bsky.embed.recordWithMedia#view"
	bskyEmbedText      = "app.bsky.embed.record#view"
	bskyEmbedTextQuote = "app.bsky.embed.record#viewRecord"
	bskyEmbedList      = "app.bsky.graph.defs#listView"
	bskyEmbedFeed      = "app.bsky.feed.defs#generatorView"
	bskyEmbedPack      = "app.bsky.graph.defs#starterPackViewBasic"
	unknownType        = "unknownType"

	modList    = "app.bsky.graph.defs#modlist"
	curateList = "app.bsky.graph.defs#curatelist"
)

var (
	sDialer = &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   sDial,
	}

	timeoutClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           sDialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       time.Minute,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
	}

	isBlueskyDead atomic.Bool

	profileTemplate = template.Must(template.ParseFiles("./views/profile.html"))
	feedTemplate    = template.Must(template.ParseFiles("./views/feed.html"))
	listTemplate    = template.Must(template.ParseFiles("./views/list.html"))
	packTemplate    = template.Must(template.ParseFiles("./views/pack.html"))
	postTemplate    = template.Must(template.New("post.html").Funcs(template.FuncMap{"escapePath": url.PathEscape, "nl2br": nl2br}).ParseFiles("./views/post.html"))
	errorTemplate   = template.Must(template.ParseFiles("./views/error.html"))
)

// Return values: string = DID, bool = ok (request succeeded or not)
func resolveHandleAPI(ctx context.Context, handle string) (string, bool) {
	apiURL := "https://public.api.bsky.app/xrpc/com.atproto.identity.resolveHandle?handle=" + handle
	if isBlueskyDead.Load() {
		apiURL = "https://api.bsky.app/xrpc/com.atproto.identity.resolveHandle?handle=" + handle
	}

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		return handle, false
	}

	resp, respErr := timeoutClient.Do(req)
	if respErr != nil {
		return handle, false
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return handle, false
	}

	var uDID apiDID
	if decodeErr := json.NewDecoder(resp.Body).Decode(&uDID); decodeErr != nil {
		return handle, false
	}

	if !strings.HasPrefix(uDID.DID, "did:") {
		return handle, false
	}

	return uDID.DID, true
}

func resolveHandleDNS(ctx context.Context, handle string) (string, bool) {
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

func resolveHandleHTTP(ctx context.Context, handle string) (string, bool) {
	atURL := fmt.Sprintf("https://%s/.well-known/atproto-did", handle)

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, atURL, http.NoBody)
	if reqErr != nil {
		return handle, false
	}

	resp, respErr := timeoutClient.Do(req)
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
func resolveHandle(ctx context.Context, handle string) string {
	// Try using the API first
	if did, ok := resolveHandleAPI(ctx, handle); ok {
		return did
	}

	// Try using DNS
	if did, ok := resolveHandleDNS(ctx, handle); ok {
		return did
	}

	// Try using .well-known
	if did, ok := resolveHandleHTTP(ctx, handle); ok {
		return did
	}

	// Failed to find DID, use the handle we got
	return handle
}

func resolvePLC(ctx context.Context, did string) plcDirectory {
	var didURL string

	// https://atproto.com/specs/did#blessed-did-methods
	if strings.HasPrefix(did, "did:plc:") {
		didURL = "https://plc.directory/" + did
	} else if didweb, ok := strings.CutPrefix(did, "did:web:"); ok {
		didURL = fmt.Sprintf("https://%s/.well-known/did.json", didweb)
	} else {
		return plcDirectory{}
	}

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, didURL, http.NoBody)
	if reqErr != nil {
		return plcDirectory{}
	}

	resp, respErr := timeoutClient.Do(req)
	if respErr != nil {
		return plcDirectory{}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return plcDirectory{}
	}

	var plc plcDirectory

	if decodeErr := json.NewDecoder(io.LimitReader(resp.Body, maxReadLimit)).Decode(&plc); decodeErr != nil {
		return plcDirectory{}
	}

	return plc
}

func getProfile(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	profileID = strings.ReplaceAll(profileID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = resolveHandle(r.Context(), editedPID)
	}
	plcData := resolvePLC(r.Context(), editedPID)

	apiURL := "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=" + editedPID
	if isBlueskyDead.Load() {
		apiURL = "https://api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=" + editedPID
	}

	req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		errorPage(w, "getProfile: Failed to create request")
		return
	}

	resp, respErr := timeoutClient.Do(req)
	if errors.Is(respErr, context.DeadlineExceeded) {
		errorPage(w, "getProfile: Bluesky took too long to respond (timeout exceeded)")
		return
	} else if respErr != nil {
		errorPage(w, "getProfile: Failed to do request")
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorPage(w, fmt.Sprintf("getProfile: Unexpected status (%s)", resp.Status))
		return
	}

	var profile userProfile
	if decodeErr := json.NewDecoder(resp.Body).Decode(&profile); decodeErr != nil {
		errorPage(w, "getProfile: Failed to decode response")
		return
	}

	if len(plcData.AKA) > 0 {
		profile.Handle = strings.TrimPrefix(plcData.AKA[0], "at://")

		if profile.DisplayName == "" {
			profile.DisplayName = profile.Handle
		}
	}

	if strings.HasPrefix(r.Host, "api.") {
		w.Header().Set("Content-Type", "application/json")

		if encodeErr := json.NewEncoder(w).Encode(&profile); encodeErr != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}

		return
	}

	isTelegramAgent := strings.Contains(r.Header.Get("User-Agent"), "Telegram")

	encodedID := richActivityEncoded{
		Type:   "prof",
		Handle: profile.Handle,
	}

	marshaled, err := json.Marshal(encodedID)
	if err != nil {
		errorPage(w, "getProfile: failed to marshal for activity")
		return
	}

	profileTemplate.Execute(w, map[string]any{"profile": profile, "isTelegram": isTelegramAgent, "encodedID": hex.EncodeToString(marshaled)})
}

func getFeed(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	feedID := r.PathValue("feedID")
	feedID = strings.ReplaceAll(feedID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = resolveHandle(r.Context(), editedPID)
	}
	plcData := resolvePLC(r.Context(), editedPID)

	if !strings.HasPrefix(editedPID, "at://") {
		editedPID = "at://" + editedPID
	}

	apiURL := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.feed.getFeedGenerator?feed=%s/app.bsky.feed.generator/%s", editedPID, feedID)
	if isBlueskyDead.Load() {
		apiURL = fmt.Sprintf("https://api.bsky.app/xrpc/app.bsky.feed.getFeedGenerator?feed=%s/app.bsky.feed.generator/%s", editedPID, feedID)
	}

	req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		errorPage(w, "getFeed: failed to create request")
		return
	}

	resp, respErr := timeoutClient.Do(req)
	if errors.Is(respErr, context.DeadlineExceeded) {
		errorPage(w, "getFeed: Bluesky took too long to respond (timeout exceeded)")
		return
	} else if respErr != nil {
		errorPage(w, "getFeed: failed to do request")
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorPage(w, fmt.Sprintf("getFeed: Unexpected status (%s)", resp.Status))
		return
	}

	var feed apiFeed
	if decodeErr := json.NewDecoder(resp.Body).Decode(&feed); decodeErr != nil {
		errorPage(w, "getFeed: failed to decode response")
		return
	}

	if len(plcData.AKA) > 0 {
		feed.View.Creator.Handle = strings.TrimPrefix(plcData.AKA[0], "at://")

		if feed.View.Creator.DisplayName == "" {
			feed.View.Creator.DisplayName = feed.View.Creator.Handle
		}
	}

	feed.View.Description = fmt.Sprintf("📡 A feed by %s (@%s)\n\n%s", feed.View.Creator.DisplayName, feed.View.Creator.Handle, feed.View.Description)

	if strings.HasPrefix(r.Host, "api.") {
		w.Header().Set("Content-Type", "application/json")

		if encodeErr := json.NewEncoder(w).Encode(&feed); encodeErr != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}

		return
	}

	isTelegramAgent := strings.Contains(r.Header.Get("User-Agent"), "Telegram")

	encodedID := richActivityEncoded{
		Type:   "feed",
		Handle: feed.View.Creator.DID,
		PostID: feedID,
	}

	marshaled, err := json.Marshal(encodedID)
	if err != nil {
		errorPage(w, "getFeed: failed to marshal for activity")
		return
	}

	feedTemplate.Execute(w, map[string]any{"feed": feed, "feedID": feedID, "isTelegram": isTelegramAgent, "encodedID": hex.EncodeToString(marshaled)})
}

func getList(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	listID := r.PathValue("listID")
	listID = strings.ReplaceAll(listID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = resolveHandle(r.Context(), editedPID)
	}
	plcData := resolvePLC(r.Context(), editedPID)

	if !strings.HasPrefix(editedPID, "at://") {
		editedPID = "at://" + editedPID
	}

	apiURL := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.graph.getList?limit=1&list=%s/app.bsky.graph.list/%s", editedPID, listID)
	if isBlueskyDead.Load() {
		apiURL = fmt.Sprintf("https://api.bsky.app/xrpc/app.bsky.graph.getList?limit=1&list=%s/app.bsky.graph.list/%s", editedPID, listID)
	}

	req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		errorPage(w, "getList: failed to create request")
		return
	}

	resp, respErr := timeoutClient.Do(req)
	if errors.Is(respErr, context.DeadlineExceeded) {
		errorPage(w, "getList: Bluesky took too long to respond (timeout exceeded)")
		return
	} else if respErr != nil {
		errorPage(w, "getList: failed to do request")
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorPage(w, fmt.Sprintf("getList: Unexpected status (%s)", resp.Status))
		return
	}

	var list apiList
	if decodeErr := json.NewDecoder(resp.Body).Decode(&list); decodeErr != nil {
		errorPage(w, "getList: failed to decode response")
		return
	}

	if len(plcData.AKA) > 0 {
		list.List.Creator.Handle = strings.TrimPrefix(plcData.AKA[0], "at://")

		if list.List.Creator.DisplayName == "" {
			list.List.Creator.DisplayName = list.List.Creator.Handle
		}
	}

	switch list.List.Purpose {
	case modList:
		list.List.Description = fmt.Sprintf("🚫 A moderation list by %s (@%s)\n\n%s", list.List.Creator.DisplayName, list.List.Creator.Handle, list.List.Description)
	case curateList:
		list.List.Description = fmt.Sprintf("👥 A curator list by %s (@%s)\n\n%s", list.List.Creator.DisplayName, list.List.Creator.Handle, list.List.Description)
	}

	if strings.HasPrefix(r.Host, "api.") {
		w.Header().Set("Content-Type", "application/json")

		if encodeErr := json.NewEncoder(w).Encode(&list); encodeErr != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}

		return
	}

	isTelegramAgent := strings.Contains(r.Header.Get("User-Agent"), "Telegram")

	encodedID := richActivityEncoded{
		Type:   "list",
		Handle: list.List.Creator.DID,
		PostID: listID,
	}

	marshaled, err := json.Marshal(encodedID)
	if err != nil {
		errorPage(w, "getList: failed to marshal for activity")
		return
	}

	listTemplate.Execute(w, map[string]any{"list": list.List, "listID": listID, "isTelegram": isTelegramAgent, "encodedID": hex.EncodeToString(marshaled)})
}

func getPack(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	packID := r.PathValue("packID")
	packID = strings.ReplaceAll(packID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = resolveHandle(r.Context(), editedPID)
	}
	plcData := resolvePLC(r.Context(), editedPID)

	if !strings.HasPrefix(editedPID, "at://") {
		editedPID = "at://" + editedPID
	}

	apiURL := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.graph.getStarterPack?starterPack=%s/app.bsky.graph.starterpack/%s", editedPID, packID)
	if isBlueskyDead.Load() {
		apiURL = fmt.Sprintf("https://api.bsky.app/xrpc/app.bsky.graph.getStarterPack?starterPack=%s/app.bsky.graph.starterpack/%s", editedPID, packID)
	}

	req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		errorPage(w, "getPack: failed to create request")
		return
	}

	resp, respErr := timeoutClient.Do(req)
	if errors.Is(respErr, context.DeadlineExceeded) {
		errorPage(w, "getPack: Bluesky took too long to respond (timeout exceeded)")
		return
	} else if respErr != nil {
		errorPage(w, "getPack: failed to do request")
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorPage(w, fmt.Sprintf("getPack: Unexpected status (%s)", resp.Status))
		return
	}

	var pack apiPack
	if decodeErr := json.NewDecoder(resp.Body).Decode(&pack); decodeErr != nil {
		errorPage(w, "getPack: failed to decode response")
		return
	}

	if len(plcData.AKA) > 0 {
		pack.StarterPack.Creator.Handle = strings.TrimPrefix(plcData.AKA[0], "at://")

		if pack.StarterPack.Creator.DisplayName == "" {
			pack.StarterPack.Creator.DisplayName = pack.StarterPack.Creator.Handle
		}
	}

	pack.StarterPack.Record.Description = fmt.Sprintf("📦 A starter pack by %s (@%s)\n\n%s", pack.StarterPack.Creator.DisplayName, pack.StarterPack.Creator.Handle, pack.StarterPack.Record.Description)

	if strings.HasPrefix(r.Host, "api.") {
		w.Header().Set("Content-Type", "application/json")

		if encodeErr := json.NewEncoder(w).Encode(&pack); encodeErr != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}

		return
	}

	isTelegramAgent := strings.Contains(r.Header.Get("User-Agent"), "Telegram")

	encodedID := richActivityEncoded{
		Type:   "pack",
		Handle: pack.StarterPack.Creator.DID,
		PostID: packID,
	}

	marshaled, err := json.Marshal(encodedID)
	if err != nil {
		errorPage(w, "getPack: failed to marshal for activity")
		return
	}

	packTemplate.Execute(w, map[string]any{"pack": pack.StarterPack, "packID": packID, "isTelegram": isTelegramAgent, "encodedID": hex.EncodeToString(marshaled)})
}

func getPost(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	postID := r.PathValue("postID")
	postID = strings.ReplaceAll(postID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = resolveHandle(r.Context(), editedPID)
	}
	plcData := resolvePLC(r.Context(), editedPID)

	if !strings.HasPrefix(editedPID, "at://") {
		editedPID = "at://" + editedPID
	}

	apiURL := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.feed.getPostThread?depth=0&uri=%s/app.bsky.feed.post/%s", editedPID, postID)
	if isBlueskyDead.Load() {
		apiURL = fmt.Sprintf("https://api.bsky.app/xrpc/app.bsky.feed.getPostThread?depth=0&uri=%s/app.bsky.feed.post/%s", editedPID, postID)
	}

	postReq, postReqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if postReqErr != nil {
		errorPage(w, "getPost: Failed to create request")
		return
	}

	postResp, postRespErr := timeoutClient.Do(postReq)
	if errors.Is(postRespErr, context.DeadlineExceeded) {
		errorPage(w, "getPost: Bluesky took too long to respond (timeout exceeded)")
		return
	} else if postRespErr != nil {
		errorPage(w, "getPost: Failed to do request")
		return
	}

	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK {
		errorPage(w, fmt.Sprintf("getPost: Unexpected status (%s)", postResp.Status))
		return
	}

	var postData apiThread

	if decodeErr := json.NewDecoder(postResp.Body).Decode(&postData); decodeErr != nil {
		errorPage(w, "getPost: Failed to decode response")
		return
	}

	// Build data here instead of in the template
	var selfData ownData

	selfData.Author = postData.Thread.Post.Author
	if len(plcData.AKA) > 0 {
		selfData.Author.Handle = strings.TrimPrefix(plcData.AKA[0], "at://")

		if selfData.Author.DisplayName == "" {
			selfData.Author.DisplayName = selfData.Author.Handle
		}
	}

	selfData.PDS = "https://bsky.social"
	selfData.Record = postData.Thread.Post.Record

	selfData.ReplyCount = postData.Thread.Post.ReplyCount
	selfData.RepostCount = postData.Thread.Post.RepostCount
	selfData.LikeCount = postData.Thread.Post.LikeCount
	selfData.QuoteCount = postData.Thread.Post.QuoteCount

	selfData.Description = selfData.Record.Text
	selfData.StatsForTG = fmt.Sprintf("💬 %s   🔁 %s   🩷 %s   📝 %s", toNotation(postData.Thread.Post.ReplyCount), toNotation(postData.Thread.Post.RepostCount), toNotation(postData.Thread.Post.LikeCount), toNotation(postData.Thread.Post.QuoteCount))

	// This is to reduce redundancy in the templates
	switch postData.Thread.Post.Embed.Type {
	case bskyEmbedImages:
		// Image(s)
		selfData.Type = bskyEmbedImages
		selfData.Images = postData.Thread.Post.Embed.Images
	case galleryImages:
		selfData.Type = galleryImages
		selfData.Images = postData.Thread.Post.Embed.Items
	case bskyEmbedExternal:
		// External
		selfData.Type = bskyEmbedExternal
		selfData.External = postData.Thread.Post.Embed.External
	case bskyEmbedVideo:
		// Video
		selfData.Type = bskyEmbedVideo
		selfData.VideoCID = postData.Thread.Post.Embed.CID
		selfData.VideoDID = postData.Thread.Post.Author.DID
		selfData.AspectRatio = postData.Thread.Post.Embed.AspectRatio
		selfData.Thumbnail = postData.Thread.Post.Embed.Thumbnail
		selfData.IsVideo = true
	case bskyEmbedQuote:
		// Quote
		switch postData.Thread.Post.Embed.Media.Type {
		case bskyEmbedImages:
			selfData.Type = bskyEmbedImages
			selfData.Images = postData.Thread.Post.Embed.Media.Images
		case galleryImages:
			selfData.Type = galleryImages
			selfData.Images = postData.Thread.Post.Embed.Media.Items
		case bskyEmbedExternal:
			selfData.Type = bskyEmbedExternal
			selfData.External = postData.Thread.Post.Embed.Media.External
		case bskyEmbedVideo:
			selfData.Type = bskyEmbedVideo
			selfData.VideoCID = postData.Thread.Post.Embed.Media.CID
			selfData.VideoDID = postData.Thread.Post.Author.DID
			selfData.AspectRatio = postData.Thread.Post.Embed.Media.AspectRatio
			selfData.Thumbnail = postData.Thread.Post.Embed.Media.Thumbnail
			selfData.IsVideo = true
		default:
			selfData.Type = unknownType
		}
	case bskyEmbedText:
		// Do we have any quote embeds?
		if len(postData.Thread.Post.Embed.Record.Embeds) > 0 {
			// Yup
			theEmbed := postData.Thread.Post.Embed.Record.Embeds[0]

			switch theEmbed.Type {
			case bskyEmbedImages:
				selfData.Type = bskyEmbedImages
				selfData.Images = theEmbed.Images
			case galleryImages:
				selfData.Type = galleryImages
				selfData.Images = theEmbed.Items
			case bskyEmbedExternal:
				selfData.Type = bskyEmbedExternal
				selfData.External = theEmbed.External
			case bskyEmbedVideo:
				selfData.Type = bskyEmbedVideo
				selfData.VideoCID = theEmbed.CID
				selfData.VideoDID = postData.Thread.Post.Embed.Record.Author.DID
				selfData.AspectRatio = theEmbed.AspectRatio
				selfData.Thumbnail = theEmbed.Thumbnail
				selfData.IsVideo = true
			case bskyEmbedQuote:
				switch theEmbed.Media.Type {
				case bskyEmbedImages:
					selfData.Type = bskyEmbedImages
					selfData.Images = theEmbed.Media.Images
				case galleryImages:
					selfData.Type = galleryImages
					selfData.Images = theEmbed.Media.Items
				case bskyEmbedExternal:
					selfData.Type = bskyEmbedExternal
					selfData.External = theEmbed.Media.External
				case bskyEmbedVideo:
					selfData.Type = bskyEmbedVideo
					selfData.VideoCID = theEmbed.Media.CID
					selfData.VideoDID = postData.Thread.Post.Embed.Record.Author.DID
					selfData.AspectRatio = theEmbed.Media.AspectRatio
					selfData.Thumbnail = theEmbed.Media.Thumbnail
					selfData.IsVideo = true
				default:
					selfData.Type = unknownType
				}
			default:
				// Text post (assumed), check if this is a list, starter pack, or a feed
				switch theEmbed.Record.Type {
				case bskyEmbedList:
					selfData.Type = bskyEmbedList
					selfData.CommonEmbeds.Name = theEmbed.Record.Name
					selfData.CommonEmbeds.Avatar = theEmbed.Record.Avatar
					selfData.CommonEmbeds.Description = theEmbed.Record.Description
					selfData.CommonEmbeds.Purpose = theEmbed.Record.Purpose
					selfData.CommonEmbeds.Creator = theEmbed.Record.Creator
				case bskyEmbedPack:
					selfData.Type = bskyEmbedPack
					selfData.CommonEmbeds.Name = theEmbed.Record.Record.Name
					selfData.CommonEmbeds.Description = theEmbed.Record.Record.Description
					selfData.CommonEmbeds.Creator = theEmbed.Record.Creator

					// Show a starter pack card. Discard before and then find the id after this --v, then construct a URL if found (ok)
					if _, packID, ok := strings.Cut(theEmbed.Record.URI, "app.bsky.graph.starterpack/"); ok {
						selfData.CommonEmbeds.Avatar = fmt.Sprintf("https://ogcard.cdn.bsky.app/start/%s/%s", theEmbed.Record.Creator.DID, packID)
					}
				case bskyEmbedFeed:
					selfData.Type = bskyEmbedFeed
					selfData.CommonEmbeds.Name = theEmbed.Record.DisplayName
					selfData.CommonEmbeds.Avatar = theEmbed.Record.Avatar
					selfData.CommonEmbeds.Description = theEmbed.Record.Description
					selfData.CommonEmbeds.Creator = theEmbed.Record.Creator
				default:
					selfData.Type = unknownType
				}
			}
		} else {
			// Nope, check if this is a list, starter pack, or a feed
			switch postData.Thread.Post.Embed.Record.Type {
			case bskyEmbedList:
				selfData.Type = bskyEmbedList
				selfData.CommonEmbeds.Name = postData.Thread.Post.Embed.Record.Name
				selfData.CommonEmbeds.Avatar = postData.Thread.Post.Embed.Record.Avatar
				selfData.CommonEmbeds.Description = postData.Thread.Post.Embed.Record.Description
				selfData.CommonEmbeds.Purpose = postData.Thread.Post.Embed.Record.Purpose
				selfData.CommonEmbeds.Creator = postData.Thread.Post.Embed.Record.Creator
			case bskyEmbedPack:
				selfData.Type = bskyEmbedPack
				selfData.CommonEmbeds.Name = postData.Thread.Post.Embed.Record.Record.Name
				selfData.CommonEmbeds.Description = postData.Thread.Post.Embed.Record.Record.Description
				selfData.CommonEmbeds.Creator = postData.Thread.Post.Embed.Record.Creator

				// Show a starter pack card. Discard before and then find the id after this --v, then construct a URL if found (ok)
				if _, packID, ok := strings.Cut(postData.Thread.Post.Embed.Record.URI, "app.bsky.graph.starterpack/"); ok {
					selfData.CommonEmbeds.Avatar = fmt.Sprintf("https://ogcard.cdn.bsky.app/start/%s/%s", postData.Thread.Post.Embed.Record.Creator.DID, packID)
				}
			case bskyEmbedFeed:
				selfData.Type = bskyEmbedFeed
				selfData.CommonEmbeds.Name = postData.Thread.Post.Embed.Record.DisplayName
				selfData.CommonEmbeds.Avatar = postData.Thread.Post.Embed.Record.Avatar
				selfData.CommonEmbeds.Description = postData.Thread.Post.Embed.Record.Description
				selfData.CommonEmbeds.Creator = postData.Thread.Post.Embed.Record.Creator
			default:
				selfData.Type = unknownType
			}
		}
	default:
		// Text post (assumed), check if parent or quote
		if postData.Thread.Parent != nil {
			// Reply
			switch postData.Thread.Parent.Post.Embed.Type {
			case bskyEmbedImages:
				selfData.Type = bskyEmbedImages
				selfData.Images = postData.Thread.Parent.Post.Embed.Images
			case galleryImages:
				selfData.Type = galleryImages
				selfData.Images = postData.Thread.Parent.Post.Embed.Items
			case bskyEmbedExternal:
				selfData.Type = bskyEmbedExternal
				selfData.External = postData.Thread.Parent.Post.Embed.External
			case bskyEmbedVideo:
				selfData.Type = bskyEmbedVideo
				selfData.VideoCID = postData.Thread.Parent.Post.Embed.CID
				selfData.VideoDID = postData.Thread.Parent.Post.Author.DID
				selfData.AspectRatio = postData.Thread.Parent.Post.Embed.AspectRatio
				selfData.Thumbnail = postData.Thread.Parent.Post.Embed.Thumbnail
				selfData.IsVideo = true
			case bskyEmbedQuote:
				switch postData.Thread.Parent.Post.Embed.Media.Type {
				case bskyEmbedImages:
					selfData.Type = bskyEmbedImages
					selfData.Images = postData.Thread.Parent.Post.Embed.Media.Images
				case galleryImages:
					selfData.Type = galleryImages
					selfData.Images = postData.Thread.Parent.Post.Embed.Items
				case bskyEmbedExternal:
					selfData.Type = bskyEmbedExternal
					selfData.External = postData.Thread.Parent.Post.Embed.Media.External
				case bskyEmbedVideo:
					selfData.Type = bskyEmbedVideo
					selfData.VideoCID = postData.Thread.Parent.Post.Embed.Media.CID
					selfData.VideoDID = postData.Thread.Parent.Post.Author.DID
					selfData.AspectRatio = postData.Thread.Parent.Post.Embed.Media.AspectRatio
					selfData.Thumbnail = postData.Thread.Parent.Post.Embed.Media.Thumbnail
					selfData.IsVideo = true
				default:
					selfData.Type = unknownType
				}
			case bskyEmbedText:
				switch postData.Thread.Parent.Post.Embed.Record.Type {
				case bskyEmbedList:
					selfData.Type = bskyEmbedList
					selfData.CommonEmbeds.Name = postData.Thread.Parent.Post.Embed.Record.Name
					selfData.CommonEmbeds.Avatar = postData.Thread.Parent.Post.Embed.Record.Avatar
					selfData.CommonEmbeds.Description = postData.Thread.Parent.Post.Embed.Record.Description
					selfData.CommonEmbeds.Purpose = postData.Thread.Parent.Post.Embed.Record.Purpose
					selfData.CommonEmbeds.Creator = postData.Thread.Parent.Post.Embed.Record.Creator
				case bskyEmbedPack:
					selfData.Type = bskyEmbedPack
					selfData.CommonEmbeds.Name = postData.Thread.Parent.Post.Embed.Record.Record.Name
					selfData.CommonEmbeds.Description = postData.Thread.Parent.Post.Embed.Record.Record.Description
					selfData.CommonEmbeds.Creator = postData.Thread.Parent.Post.Embed.Record.Creator

					// Show a starter pack card. Discard before and then find the id after this --v, then construct a URL if found (ok)
					if _, packID, ok := strings.Cut(postData.Thread.Parent.Post.Embed.Record.URI, "app.bsky.graph.starterpack/"); ok {
						selfData.CommonEmbeds.Avatar = fmt.Sprintf("https://ogcard.cdn.bsky.app/start/%s/%s", postData.Thread.Parent.Post.Embed.Record.Creator.DID, packID)
					}
				case bskyEmbedFeed:
					selfData.Type = bskyEmbedFeed
					selfData.CommonEmbeds.Name = postData.Thread.Parent.Post.Embed.Record.DisplayName
					selfData.CommonEmbeds.Avatar = postData.Thread.Parent.Post.Embed.Record.Avatar
					selfData.CommonEmbeds.Description = postData.Thread.Parent.Post.Embed.Record.Description
					selfData.CommonEmbeds.Creator = postData.Thread.Parent.Post.Embed.Record.Creator
				default:
					selfData.Type = unknownType
				}
			default:
				selfData.Type = unknownType
			}
		} else {
			selfData.Type = unknownType
		}
	}

	var mediaMsg string
	switch selfData.Type {
	case bskyEmbedList:
		if selfData.CommonEmbeds.Creator.DisplayName == "" {
			selfData.CommonEmbeds.Creator.DisplayName = selfData.CommonEmbeds.Creator.Handle
		}

		switch selfData.CommonEmbeds.Purpose {
		case modList:
			selfData.Description += fmt.Sprintf("\n\n%s\n🚫 A moderation list by %s (@%s)\n\n%s", selfData.CommonEmbeds.Name, selfData.CommonEmbeds.Creator.DisplayName, selfData.CommonEmbeds.Creator.Handle, selfData.CommonEmbeds.Description)
		case curateList:
			selfData.Description += fmt.Sprintf("\n\n%s\n👥 A curator list by %s (@%s)\n\n%s", selfData.CommonEmbeds.Name, selfData.CommonEmbeds.Creator.DisplayName, selfData.CommonEmbeds.Creator.Handle, selfData.CommonEmbeds.Description)
		}
	case bskyEmbedPack:
		if selfData.CommonEmbeds.Creator.DisplayName == "" {
			selfData.CommonEmbeds.Creator.DisplayName = selfData.CommonEmbeds.Creator.Handle
		}

		selfData.Description += fmt.Sprintf("\n\n%s\n📦 A starter pack by %s (@%s)\n\n%s", selfData.CommonEmbeds.Name, selfData.CommonEmbeds.Creator.DisplayName, selfData.CommonEmbeds.Creator.Handle, selfData.CommonEmbeds.Description)
	case bskyEmbedFeed:
		if selfData.CommonEmbeds.Creator.DisplayName == "" {
			selfData.CommonEmbeds.Creator.DisplayName = selfData.CommonEmbeds.Creator.Handle
		}

		selfData.Description += fmt.Sprintf("\n\n%s\n📡 A feed by %s (@%s)\n\n%s", selfData.CommonEmbeds.Name, selfData.CommonEmbeds.Creator.DisplayName, selfData.CommonEmbeds.Creator.Handle, selfData.CommonEmbeds.Description)
	case bskyEmbedExternal:
		parsedURL, parseErr := url.Parse(selfData.External.URI)
		if parseErr != nil {
			// Let's assume it's not a gif
			selfData.IsGif = false
		} else {
			selfData.IsGif = (parsedURL.Host == "media.tenor.com")
		}

		if selfData.IsGif {
			// The template is stupidly persistent on rewriting & to &amp; come hell or high water it will rewrite it
			selfData.External.URI = "https://media.tenor.com" + parsedURL.Path
		} else {
			// Not a GIF, Add the external's title & description to the template description
			selfData.Description += "\n\n" + selfData.External.Title + "\n" + selfData.External.Description
		}
	case bskyEmbedImages, galleryImages:
		pnStr := r.PathValue("photoNum")
		if pnStr != "" {
			pnValue, atoiErr := strconv.Atoi(pnStr)
			if atoiErr != nil {
				errorPage(w, "getPost: Invalid photo number")
				return
			}

			if pnValue < 1 {
				pnValue = 1
			}

			imgLen := len(selfData.Images)
			if imgLen > 1 && imgLen >= pnValue {
				mediaMsg = fmt.Sprintf("Photo %d of %d", pnValue, imgLen)
				selfData.Images = apiImages{selfData.Images[pnValue-1]}
			}
		}
	case bskyEmbedVideo:
		vidOwnerPLC := resolvePLC(r.Context(), selfData.VideoDID)
		for _, k := range vidOwnerPLC.Service {
			if k.ID == "#atproto_pds" && k.Type == "AtprotoPersonalDataServer" {
				selfData.PDS = k.Endpoint
				break
			}
		}
	}

	// Add description details, could be done in the switch above, but it's easier to find it here.
	// Prioritize quoting first, then replies.
	switch postData.Thread.Post.Embed.Type {
	case bskyEmbedText:
		if postData.Thread.Post.Embed.Record.Type == bskyEmbedTextQuote {
			if selfData.Description != "" {
				selfData.Description += "\n\n"
			}

			if postData.Thread.Post.Embed.Record.Author.DisplayName == "" {
				postData.Thread.Post.Embed.Record.Author.DisplayName = postData.Thread.Post.Embed.Record.Author.Handle
			}

			_, qPID, found := strings.Cut(postData.Thread.Post.Embed.Record.URI, "app.bsky.feed.post/")
			if found {
				selfData.OriginalPostID = qPID
			}

			selfData.Description += fmt.Sprintf("📝 Quoting %s (@%s):\n%s", postData.Thread.Post.Embed.Record.Author.DisplayName, postData.Thread.Post.Embed.Record.Author.Handle, postData.Thread.Post.Embed.Record.Value.Text)
		}
	case bskyEmbedQuote:
		if selfData.Description != "" {
			selfData.Description += "\n\n"
		}

		if postData.Thread.Post.Embed.Record.Record.Author.DisplayName == "" {
			postData.Thread.Post.Embed.Record.Record.Author.DisplayName = postData.Thread.Post.Embed.Record.Record.Author.Handle
		}

		_, qPID, found := strings.Cut(postData.Thread.Post.Embed.Record.Record.URI, "app.bsky.feed.post/")
		if found {
			selfData.OriginalPostID = qPID
		}

		selfData.Description += fmt.Sprintf("📝 Quoting %s (@%s):\n%s", postData.Thread.Post.Embed.Record.Record.Author.DisplayName, postData.Thread.Post.Embed.Record.Record.Author.Handle, postData.Thread.Post.Embed.Record.Record.Value.Text)
	}

	if postData.Thread.Parent != nil {
		if selfData.Description != "" {
			selfData.Description += "\n\n"
		}

		if postData.Thread.Parent.Post.Author.DisplayName == "" {
			postData.Thread.Parent.Post.Author.DisplayName = postData.Thread.Parent.Post.Author.Handle
		}

		_, qPID, found := strings.Cut(postData.Thread.Parent.Post.URI, "app.bsky.feed.post/")
		if found {
			selfData.OriginalPostID = qPID
		}

		selfData.Description += fmt.Sprintf("💬 Replying to %s (@%s):\n%s", postData.Thread.Parent.Post.Author.DisplayName, postData.Thread.Parent.Post.Author.Handle, postData.Thread.Parent.Post.Record.Text)
	}

	if strings.HasPrefix(r.Host, "mosaic.") {
		if selfData.Type == bskyEmbedImages || selfData.Type == galleryImages {
			genMosaic(w, r, selfData.Images)
			return
		}

		errorPage(w, "getPost: Invalid type")
		return
	}

	if strings.HasPrefix(r.Host, "raw.") {
		switch selfData.Type {
		case bskyEmbedImages, galleryImages:
			genMosaic(w, r, selfData.Images)
			return
		case bskyEmbedExternal:
			if selfData.IsGif {
				http.Redirect(w, r, selfData.External.URI, http.StatusFound)
				return
			}

			if selfData.External.Thumb != "" {
				http.Redirect(w, r, selfData.External.Thumb, http.StatusFound)
				return
			}

			errorPage(w, "getPost: No suitable media found")
			return
		case bskyEmbedVideo:
			http.Redirect(w, r, fmt.Sprintf("%s/xrpc/com.atproto.sync.getBlob?cid=%s&did=%s", selfData.PDS, selfData.VideoCID, selfData.VideoDID), http.StatusFound)
			return
		case bskyEmbedList, bskyEmbedPack, bskyEmbedFeed:
			if selfData.CommonEmbeds.Avatar != "" {
				http.Redirect(w, r, selfData.CommonEmbeds.Avatar, http.StatusFound)
				return
			}

			errorPage(w, "getPost: No suitable media found")
			return
		default:
			errorPage(w, "getPost: Invalid type")
			return
		}
	}

	if strings.HasPrefix(r.Host, "api.") {
		if selfData.Type == bskyEmbedVideo {
			selfData.VideoHelper = fmt.Sprintf("%s/xrpc/com.atproto.sync.getBlob?cid=%s&did=%s", selfData.PDS, selfData.VideoCID, selfData.VideoDID)
		}

		var buf bytes.Buffer
		if encodeErr := json.NewEncoder(&buf).Encode(map[string]any{"originalData": postData, "parsedData": selfData}); encodeErr != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(buf.Bytes())
		return
	}

	isTelegramAgent := strings.Contains(r.Header.Get("User-Agent"), "Telegram")

	encodedID := richActivityEncoded{
		Type:     "post",
		Handle:   selfData.Author.DID,
		PostID:   postID,
		PhotoCut: r.PathValue("photoNum"),
	}

	marshaled, err := json.Marshal(encodedID)
	if err != nil {
		errorPage(w, "getPost: failed to marshal for activity")
		return
	}

	postTemplate.Execute(w, map[string]any{"data": selfData, "editedPID": strings.TrimPrefix(editedPID, "at://"), "postID": postID, "isTelegram": isTelegramAgent, "mediaMsg": mediaMsg, "encodedID": hex.EncodeToString(marshaled)})
}

func genMosaic(w http.ResponseWriter, r *http.Request, images apiImages) {
	switch len(images) {
	case 0:
		errorPage(w, "genMosaic: No images")
		return
	case 1:
		http.Redirect(w, r, images[0].FullSize, http.StatusFound)
		return
	}

	w.Header().Set("Content-Type", "image/jpeg")

	var args []string
	var avgWidth int
	for _, k := range images {
		args = append(args, "-i", k.FullSize)
		avgWidth += int(k.AspectRatio.Width)
	}

	avgWidth /= len(images)

	var filterComplex strings.Builder
	for i := range images {
		fmt.Fprintf(&filterComplex, "[%d:v]scale=%d:-2[m%d];", i, avgWidth, i)
	}

	for i := range images {
		fmt.Fprintf(&filterComplex, "[m%d]", i)
	}
	fmt.Fprintf(&filterComplex, "hstack=inputs=%d", len(images))

	args = append(args, "-filter_complex", filterComplex.String(), "-f", "image2pipe", "-c:v", "mjpeg", "pipe:1")

	//nolint:gosec // This is just ffmpeg, with the only external values being k.FullSize, which is from the API
	cmd := exec.CommandContext(r.Context(), "ffmpeg", args...)
	cmd.Stdout = w

	if runErr := cmd.Run(); runErr != nil {
		http.Error(w, "genMosaic: Failed to run", http.StatusInternalServerError)
		return
	}
}

// source: https://compiles.me/blog/making-rich-url-embeds-for-discord && https://embedl.ink/
// thanks!
func genActivity(w http.ResponseWriter, r *http.Request) {
	encodedID := r.PathValue("id")

	hBytes, err := hex.DecodeString(encodedID)
	if err != nil {
		errorPage(w, "invalid ID")
		return
	}

	var actReqData richActivityEncoded
	if unmarshalErr := json.Unmarshal(hBytes, &actReqData); unmarshalErr != nil {
		errorPage(w, "failed to unmarshal JSON")
		return
	}

	var richEmbed richActivity
	switch actReqData.Type {
	case "post":
		apiURL := fmt.Sprintf("https://api.xbsky.app/profile/%s/post/%s", actReqData.Handle, actReqData.PostID)
		if actReqData.PhotoCut != "" {
			apiURL = fmt.Sprintf("https://api.xbsky.app/profile/%s/post/%s/photo/%s", actReqData.Handle, actReqData.PostID, actReqData.PhotoCut)
		}

		newAPIReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
		if err != nil {
			errorPage(w, "failed to request api data")
			return
		}

		apiResp, err := timeoutClient.Do(newAPIReq)
		if err != nil {
			errorPage(w, "failed to do api request")
			return
		}

		defer apiResp.Body.Close()

		var sortedAPI SortedAPIResponse

		if decodeErr := json.NewDecoder(apiResp.Body).Decode(&sortedAPI); decodeErr != nil {
			errorPage(w, "failed to decode response")
			return
		}

		var richContent string

		switch sortedAPI.OriginalData.Thread.Post.Embed.Type {
		case bskyEmbedText:
			if sortedAPI.OriginalData.Thread.Post.Embed.Record.Type == bskyEmbedTextQuote {
				if sortedAPI.OriginalData.Thread.Post.Embed.Record.Author.DisplayName == "" {
					sortedAPI.OriginalData.Thread.Post.Embed.Record.Author.DisplayName = sortedAPI.OriginalData.Thread.Post.Embed.Record.Author.Handle
				}

				var richBuilder strings.Builder

				qText := sortedAPI.OriginalData.Thread.Post.Embed.Record.Value.Text
				if qText != "" {
					if len(sortedAPI.OriginalData.Thread.Post.Embed.Record.Value.Facets) > 0 {
						richBuilder.WriteString("<p>")

						var lastByteIndex int64
						for _, v := range sortedAPI.OriginalData.Thread.Post.Embed.Record.Value.Facets {
							if len(v.Features) > 0 {
								richBuilder.WriteString(qText[lastByteIndex:v.Index.ByteStart])

								switch v.Features[0].Type {
								case "app.bsky.richtext.facet#tag":
									fmt.Fprintf(&richBuilder, `<a href=%q>#%s</a>`, "https://bsky.app/hashtag/"+v.Features[0].Tag, v.Features[0].Tag)
								case "app.bsky.richtext.facet#link":
									fmt.Fprintf(&richBuilder, `<a href=%q>%s</a>`, v.Features[0].URI, qText[v.Index.ByteStart:v.Index.ByteEnd])
								case "app.bsky.richtext.facet#mention":
									fmt.Fprintf(&richBuilder, `<a href=%q>%s</a>`, "https://bsky.app/profile/"+v.Features[0].DID, qText[v.Index.ByteStart:v.Index.ByteEnd])
								}

								lastByteIndex = v.Index.ByteEnd
							}
						}

						richBuilder.WriteString(qText[lastByteIndex:])
						richBuilder.WriteString("</p>")
					} else {
						fmt.Fprintf(&richBuilder, "<p>%s</p>", qText)
					}
				}

				richContent += fmt.Sprintf(`<b><span><a href="https://bsky.app/profile/%s/post/%s">📝 Quoting %s (@%s):</a></span></b><blockquote>%s</blockquote>`, sortedAPI.OriginalData.Thread.Post.Embed.Record.Author.DID, sortedAPI.ParsedData.OriginalPostID, sortedAPI.OriginalData.Thread.Post.Embed.Record.Author.DisplayName, sortedAPI.OriginalData.Thread.Post.Embed.Record.Author.Handle, richBuilder.String())
			}
		case bskyEmbedQuote:
			if sortedAPI.OriginalData.Thread.Post.Embed.Record.Record.Author.DisplayName == "" {
				sortedAPI.OriginalData.Thread.Post.Embed.Record.Record.Author.DisplayName = sortedAPI.OriginalData.Thread.Post.Embed.Record.Record.Author.Handle
			}

			var richBuilder strings.Builder
			qText := sortedAPI.OriginalData.Thread.Post.Embed.Record.Record.Value.Text
			if qText != "" {
				if len(sortedAPI.OriginalData.Thread.Post.Embed.Record.Record.Value.Facets) > 0 {
					richBuilder.WriteString("<p>")

					var lastByteIndex int64
					for _, v := range sortedAPI.OriginalData.Thread.Post.Embed.Record.Record.Value.Facets {
						if len(v.Features) > 0 {
							richBuilder.WriteString(qText[lastByteIndex:v.Index.ByteStart])

							switch v.Features[0].Type {
							case "app.bsky.richtext.facet#tag":
								fmt.Fprintf(&richBuilder, `<a href=%q>#%s</a>`, "https://bsky.app/hashtag/"+v.Features[0].Tag, v.Features[0].Tag)
							case "app.bsky.richtext.facet#link":
								fmt.Fprintf(&richBuilder, `<a href=%q>%s</a>`, v.Features[0].URI, qText[v.Index.ByteStart:v.Index.ByteEnd])
							case "app.bsky.richtext.facet#mention":
								fmt.Fprintf(&richBuilder, `<a href=%q>%s</a>`, "https://bsky.app/profile/"+v.Features[0].DID, qText[v.Index.ByteStart:v.Index.ByteEnd])
							}

							lastByteIndex = v.Index.ByteEnd
						}
					}

					richBuilder.WriteString(qText[lastByteIndex:])
					richBuilder.WriteString("</p>")
				} else {
					fmt.Fprintf(&richBuilder, "<p>%s</p>", qText)
				}
			}

			richContent += fmt.Sprintf(`<b><span><a href="https://bsky.app/profile/%s/post/%s">📝 Quoting %s (@%s):</a></span></b><blockquote>%s</blockquote>`, sortedAPI.OriginalData.Thread.Post.Embed.Record.Record.Author.DID, sortedAPI.ParsedData.OriginalPostID, sortedAPI.OriginalData.Thread.Post.Embed.Record.Record.Author.DisplayName, sortedAPI.OriginalData.Thread.Post.Embed.Record.Record.Author.Handle, richBuilder.String())
		}

		if sortedAPI.OriginalData.Thread.Parent != nil {
			if sortedAPI.OriginalData.Thread.Parent.Post.Author.DisplayName == "" {
				sortedAPI.OriginalData.Thread.Parent.Post.Author.DisplayName = sortedAPI.OriginalData.Thread.Parent.Post.Author.Handle
			}

			var richBuilder strings.Builder
			qText := sortedAPI.OriginalData.Thread.Parent.Post.Record.Text
			if qText != "" {
				if len(sortedAPI.OriginalData.Thread.Parent.Post.Record.Facets) > 0 {
					richBuilder.WriteString("<p>")

					var lastByteIndex int64
					for _, v := range sortedAPI.OriginalData.Thread.Parent.Post.Record.Facets {
						if len(v.Features) > 0 {
							richBuilder.WriteString(qText[lastByteIndex:v.Index.ByteStart])

							switch v.Features[0].Type {
							case "app.bsky.richtext.facet#tag":
								fmt.Fprintf(&richBuilder, `<a href=%q>#%s</a>`, "https://bsky.app/hashtag/"+v.Features[0].Tag, v.Features[0].Tag)
							case "app.bsky.richtext.facet#link":
								fmt.Fprintf(&richBuilder, `<a href=%q>%s</a>`, v.Features[0].URI, qText[v.Index.ByteStart:v.Index.ByteEnd])
							case "app.bsky.richtext.facet#mention":
								fmt.Fprintf(&richBuilder, `<a href=%q>%s</a>`, "https://bsky.app/profile/"+v.Features[0].DID, qText[v.Index.ByteStart:v.Index.ByteEnd])
							}

							lastByteIndex = v.Index.ByteEnd
						}
					}

					richBuilder.WriteString(qText[lastByteIndex:])
					richBuilder.WriteString("</p>")
				} else {
					fmt.Fprintf(&richBuilder, "<p>%s</p>", qText)
				}
			}

			richContent += fmt.Sprintf(`<span><b><a href="https://bsky.app/profile/%s/post/%s">💬 Replying to %s (@%s):</a></b></span><blockquote>%s</blockquote>`, sortedAPI.OriginalData.Thread.Parent.Post.Author.DID, sortedAPI.ParsedData.OriginalPostID, sortedAPI.OriginalData.Thread.Parent.Post.Author.DisplayName, sortedAPI.OriginalData.Thread.Parent.Post.Author.Handle, richBuilder.String())
		}

		var richBuilder strings.Builder
		if sortedAPI.ParsedData.Record.Text != "" {
			if len(sortedAPI.OriginalData.Thread.Post.Record.Facets) > 0 {
				richBuilder.WriteString("<p>")

				var lastByteIndex int64
				for _, v := range sortedAPI.OriginalData.Thread.Post.Record.Facets {
					if len(v.Features) > 0 {
						richBuilder.WriteString(sortedAPI.ParsedData.Record.Text[lastByteIndex:v.Index.ByteStart])

						switch v.Features[0].Type {
						case "app.bsky.richtext.facet#tag":
							fmt.Fprintf(&richBuilder, `<a href=%q>#%s</a>`, "https://bsky.app/hashtag/"+v.Features[0].Tag, v.Features[0].Tag)
						case "app.bsky.richtext.facet#link":
							fmt.Fprintf(&richBuilder, `<a href=%q>%s</a>`, v.Features[0].URI, sortedAPI.ParsedData.Record.Text[v.Index.ByteStart:v.Index.ByteEnd])
						case "app.bsky.richtext.facet#mention":
							fmt.Fprintf(&richBuilder, `<a href=%q>%s</a>`, "https://bsky.app/profile/"+v.Features[0].DID, sortedAPI.ParsedData.Record.Text[v.Index.ByteStart:v.Index.ByteEnd])
						}

						lastByteIndex = v.Index.ByteEnd
					}
				}

				richBuilder.WriteString(sortedAPI.ParsedData.Record.Text[lastByteIndex:])
				richBuilder.WriteString("</p>")
			} else {
				fmt.Fprintf(&richBuilder, "<p>%s</p>", sortedAPI.ParsedData.Record.Text)
			}

			richContent += richBuilder.String()
		}

		if sortedAPI.ParsedData.CommonEmbeds.Name != "" {
			richContent += "<blockquote>"

			richContent += fmt.Sprintf("<span>%s</span><br>", sortedAPI.ParsedData.CommonEmbeds.Name)

			switch sortedAPI.ParsedData.Type {
			case bskyEmbedList:
				switch sortedAPI.ParsedData.CommonEmbeds.Purpose {
				case modList:
					richContent += fmt.Sprintf(`<p><b>🚫 A moderation list by <a href="https://bsky.app/profile/%s">%s (@%s)</a></b></p>`, sortedAPI.ParsedData.CommonEmbeds.Creator.DID, sortedAPI.ParsedData.CommonEmbeds.Creator.DisplayName, sortedAPI.ParsedData.CommonEmbeds.Creator.Handle)
				case curateList:
					richContent += fmt.Sprintf(`<p><b>👥 A curator list by <a href="https://bsky.app/profile/%s">%s (@%s)</a></b></p>`, sortedAPI.ParsedData.CommonEmbeds.Creator.DID, sortedAPI.ParsedData.CommonEmbeds.Creator.DisplayName, sortedAPI.ParsedData.CommonEmbeds.Creator.Handle)
				}
			case bskyEmbedPack:
				richContent += fmt.Sprintf(`<p><b>📦 A starter pack by <a href="https://bsky.app/profile/%s">%s (@%s)</a></b></p>`, sortedAPI.ParsedData.CommonEmbeds.Creator.DID, sortedAPI.ParsedData.CommonEmbeds.Creator.DisplayName, sortedAPI.ParsedData.CommonEmbeds.Creator.Handle)
			case bskyEmbedFeed:
				richContent += fmt.Sprintf(`<p><b>📡 A feed by <a href="https://bsky.app/profile/%s">%s (@%s)</a></b></p>`, sortedAPI.ParsedData.CommonEmbeds.Creator.DID, sortedAPI.ParsedData.CommonEmbeds.Creator.DisplayName, sortedAPI.ParsedData.CommonEmbeds.Creator.Handle)
			}

			if sortedAPI.ParsedData.CommonEmbeds.Description != "" {
				richContent += fmt.Sprintf("<span>%s</span>", sortedAPI.ParsedData.CommonEmbeds.Description)
			}

			richContent += "</blockquote>"
		}

		if sortedAPI.ParsedData.External.Title != "" {
			richContent += fmt.Sprintf(`<blockquote><p>%s</p><p>%s</p><p><a href=%q>%s</a></p></blockquote>`, sortedAPI.ParsedData.External.Title, sortedAPI.ParsedData.External.Description, sortedAPI.ParsedData.External.URI, sortedAPI.ParsedData.External.URI)
		}

		richContent = strings.ReplaceAll(richContent, "\n", "<br>")

		richContent += fmt.Sprintf("<p>%s</p>", fmt.Sprintf("💬 %s &ensp; 🔁 %s &ensp; 🩷 %s &ensp; 📝 %s", toNotation(sortedAPI.ParsedData.ReplyCount), toNotation(sortedAPI.ParsedData.RepostCount), toNotation(sortedAPI.ParsedData.LikeCount), toNotation(sortedAPI.ParsedData.QuoteCount)))

		richEmbed = richActivity{
			ID:          strconv.Itoa(rand.Int()),
			URL:         fmt.Sprintf("https://bsky.app/profile/%s/post/%s", actReqData.Handle, actReqData.PostID),
			URI:         fmt.Sprintf("https://bsky.app/profile/%s/post/%s", actReqData.Handle, actReqData.PostID),
			CreatedAt:   sortedAPI.ParsedData.Record.CreatedAt,
			Language:    "en",
			Content:     richContent,
			SpoilerText: "",
			Visibility:  "public",
			Application: richActivityApplication{
				Name:    "xbsky.app",
				Website: "https://xbsky.app",
			},
			Account: richActivityAccount{
				ID:           strconv.Itoa(rand.Int()),
				DisplayName:  sortedAPI.ParsedData.Author.DisplayName,
				UserName:     sortedAPI.ParsedData.Author.Handle,
				Acct:         sortedAPI.ParsedData.Author.Handle,
				URL:          "https://bsky.app/profile/" + sortedAPI.ParsedData.Author.Handle,
				URI:          "https://bsky.app/profile/" + sortedAPI.ParsedData.Author.Handle,
				Avatar:       sortedAPI.ParsedData.Author.Avatar,
				AvatarStatic: sortedAPI.ParsedData.Author.Avatar,
			},
			MediaAttachments: []richActivityMedia{},
		}

		if sortedAPI.ParsedData.IsVideo {
			richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, richActivityMedia{
				ID:          strconv.Itoa(rand.Int()),
				Type:        "video",
				URL:         sortedAPI.ParsedData.VideoHelper,
				Preview:     sortedAPI.ParsedData.Thumbnail,
				Description: "",
			})
		} else {
			if len(sortedAPI.ParsedData.Images) > 0 {
				for _, v := range sortedAPI.ParsedData.Images {
					richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, richActivityMedia{
						ID:          strconv.Itoa(rand.Int()),
						Type:        "image",
						URL:         v.FullSize,
						Preview:     v.FullSize,
						Description: v.Alt,
					})
				}
			} else if sortedAPI.ParsedData.External.Thumb != "" {
				richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, richActivityMedia{
					ID:          strconv.Itoa(rand.Int()),
					Type:        "image",
					URL:         sortedAPI.ParsedData.External.Thumb,
					Preview:     sortedAPI.ParsedData.External.Thumb,
					Description: "",
				})
			} else if sortedAPI.ParsedData.CommonEmbeds.Avatar != "" {
				richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, richActivityMedia{
					ID:          strconv.Itoa(rand.Int()),
					Type:        "image",
					URL:         sortedAPI.ParsedData.CommonEmbeds.Avatar,
					Preview:     sortedAPI.ParsedData.CommonEmbeds.Avatar,
					Description: "",
				})
			}
		}
	case "prof":
		newAPIReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "https://api.xbsky.app/profile/"+actReqData.Handle, http.NoBody)
		if err != nil {
			errorPage(w, "failed to request api data")
			return
		}

		apiResp, err := timeoutClient.Do(newAPIReq)
		if err != nil {
			errorPage(w, "failed to do api request")
			return
		}

		defer apiResp.Body.Close()

		var sortedAPI userProfile

		if decodeErr := json.NewDecoder(apiResp.Body).Decode(&sortedAPI); decodeErr != nil {
			errorPage(w, "failed to decode response")
			return
		}

		richContent := fmt.Sprintf("<p>%s</p><p>👥 %s Followers &ensp; 🌐 %s Following &ensp; ✍️ %s Posts</p>", sortedAPI.Description, toNotation(sortedAPI.FollowersCount), toNotation(sortedAPI.FollowsCount), toNotation(sortedAPI.PostsCount))

		if sortedAPI.Associated.Labeler {
			richContent += "<p>🏷️ Labeler</p>"
		}

		richContent = strings.ReplaceAll(richContent, "\n", "<br>")

		richEmbed = richActivity{
			ID:          strconv.Itoa(rand.Int()),
			URL:         "https://bsky.app/profile/" + sortedAPI.Handle,
			URI:         "https://bsky.app/profile/" + sortedAPI.Handle,
			CreatedAt:   sortedAPI.CreatedAt,
			Language:    "en",
			Content:     richContent,
			SpoilerText: "",
			Visibility:  "public",
			Application: richActivityApplication{
				Name:    "xbsky.app",
				Website: "https://xbsky.app",
			},
			Account: richActivityAccount{
				ID:           strconv.Itoa(rand.Int()),
				DisplayName:  sortedAPI.DisplayName,
				UserName:     sortedAPI.Handle,
				Acct:         sortedAPI.Handle,
				URL:          "https://bsky.app/profile/" + sortedAPI.Handle,
				URI:          "https://bsky.app/profile/" + sortedAPI.Handle,
				Avatar:       sortedAPI.Avatar,
				AvatarStatic: sortedAPI.Avatar,
			},
			MediaAttachments: []richActivityMedia{},
		}
	case "feed":
		newAPIReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fmt.Sprintf("https://api.xbsky.app/profile/%s/feed/%s", actReqData.Handle, actReqData.PostID), http.NoBody)
		if err != nil {
			errorPage(w, "failed to request api data")
			return
		}

		apiResp, err := timeoutClient.Do(newAPIReq)
		if err != nil {
			errorPage(w, "failed to do api request")
			return
		}

		defer apiResp.Body.Close()

		var sortedAPI apiFeed

		if decodeErr := json.NewDecoder(apiResp.Body).Decode(&sortedAPI); decodeErr != nil {
			errorPage(w, "failed to decode response")
			return
		}

		richContent := fmt.Sprintf("<p>%s</p><p>🩷 %s Likes</p>", sortedAPI.View.Description, toNotation(sortedAPI.View.LikeCount))

		if sortedAPI.IsOnline {
			richContent += "<p>✅ Online</p>"
		} else {
			richContent += "<p>❌ Not online</p>"
		}

		if sortedAPI.IsValid {
			richContent += "<p>✅ Valid</p>"
		} else {
			richContent += "<p>❌ Not valid</p>"
		}

		richContent = strings.ReplaceAll(richContent, "\n", "<br>")

		richEmbed = richActivity{
			ID:          strconv.Itoa(rand.Int()),
			URL:         fmt.Sprintf("https://bsky.app/profile/%s/feed/%s", sortedAPI.View.Creator.Handle, actReqData.PostID),
			URI:         fmt.Sprintf("https://bsky.app/profile/%s/feed/%s", sortedAPI.View.Creator.Handle, actReqData.PostID),
			CreatedAt:   sortedAPI.View.IndexedAt,
			Language:    "en",
			Content:     richContent,
			SpoilerText: "",
			Visibility:  "public",
			Application: richActivityApplication{
				Name:    "xbsky.app",
				Website: "https://xbsky.app",
			},
			Account: richActivityAccount{
				ID:           strconv.Itoa(rand.Int()),
				DisplayName:  sortedAPI.View.DisplayName,
				UserName:     sortedAPI.View.Creator.Handle,
				Acct:         sortedAPI.View.Creator.Handle,
				URL:          "https://bsky.app/profile/" + sortedAPI.View.Creator.Handle,
				URI:          "https://bsky.app/profile/" + sortedAPI.View.Creator.Handle,
				Avatar:       sortedAPI.View.Creator.Avatar,
				AvatarStatic: sortedAPI.View.Creator.Avatar,
			},
			MediaAttachments: []richActivityMedia{},
		}

		if sortedAPI.View.Avatar != "" {
			richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, richActivityMedia{
				ID:          strconv.Itoa(rand.Int()),
				Type:        "image",
				URL:         sortedAPI.View.Avatar,
				Preview:     sortedAPI.View.Avatar,
				Description: "",
			})
		}
	case "list":
		newAPIReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fmt.Sprintf("https://api.xbsky.app/profile/%s/lists/%s", actReqData.Handle, actReqData.PostID), http.NoBody)
		if err != nil {
			errorPage(w, "failed to request api data")
			return
		}

		apiResp, err := timeoutClient.Do(newAPIReq)
		if err != nil {
			errorPage(w, "failed to do api request")
			return
		}

		defer apiResp.Body.Close()

		var sortedAPI apiList

		if decodeErr := json.NewDecoder(apiResp.Body).Decode(&sortedAPI); decodeErr != nil {
			errorPage(w, "failed to decode response")
			return
		}

		richContent := fmt.Sprintf("<p>%s</p><p>👥 %s on List</p>", sortedAPI.List.Description, toNotation(sortedAPI.List.ItemCount))

		richContent = strings.ReplaceAll(richContent, "\n", "<br>")

		richEmbed = richActivity{
			ID:          strconv.Itoa(rand.Int()),
			URL:         fmt.Sprintf("https://bsky.app/profile/%s/feed/%s", sortedAPI.List.Creator.Handle, actReqData.PostID),
			URI:         fmt.Sprintf("https://bsky.app/profile/%s/feed/%s", sortedAPI.List.Creator.Handle, actReqData.PostID),
			CreatedAt:   sortedAPI.List.IndexedAt,
			Language:    "en",
			Content:     richContent,
			SpoilerText: "",
			Visibility:  "public",
			Application: richActivityApplication{
				Name:    "xbsky.app",
				Website: "https://xbsky.app",
			},
			Account: richActivityAccount{
				ID:           strconv.Itoa(rand.Int()),
				DisplayName:  sortedAPI.List.Creator.DisplayName,
				UserName:     sortedAPI.List.Creator.Handle,
				Acct:         sortedAPI.List.Creator.Handle,
				URL:          "https://bsky.app/profile/" + sortedAPI.List.Creator.Handle,
				URI:          "https://bsky.app/profile/" + sortedAPI.List.Creator.Handle,
				Avatar:       sortedAPI.List.Creator.Avatar,
				AvatarStatic: sortedAPI.List.Creator.Avatar,
			},
			MediaAttachments: []richActivityMedia{},
		}

		if sortedAPI.List.Avatar != "" {
			richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, richActivityMedia{
				ID:          strconv.Itoa(rand.Int()),
				Type:        "image",
				URL:         sortedAPI.List.Avatar,
				Preview:     sortedAPI.List.Avatar,
				Description: "",
			})
		}
	case "pack":
		newAPIReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fmt.Sprintf("https://api.xbsky.app/starter-pack/%s/%s", actReqData.Handle, actReqData.PostID), http.NoBody)
		if err != nil {
			errorPage(w, "failed to request api data")
			return
		}

		apiResp, err := timeoutClient.Do(newAPIReq)
		if err != nil {
			errorPage(w, "failed to do api request")
			return
		}

		defer apiResp.Body.Close()

		var sortedAPI apiPack

		if decodeErr := json.NewDecoder(apiResp.Body).Decode(&sortedAPI); decodeErr != nil {
			errorPage(w, "failed to decode response")
			return
		}

		richContent := fmt.Sprintf("<p>%s</p>", sortedAPI.StarterPack.Record.Description)

		richContent = strings.ReplaceAll(richContent, "\n", "<br>")

		richEmbed = richActivity{
			ID:          strconv.Itoa(rand.Int()),
			URL:         fmt.Sprintf("https://bsky.app/starter-pack/%s/%s", sortedAPI.StarterPack.Creator.Handle, actReqData.PostID),
			URI:         fmt.Sprintf("https://bsky.app/starter-pack/%s/%s", sortedAPI.StarterPack.Creator.Handle, actReqData.PostID),
			CreatedAt:   sortedAPI.StarterPack.Record.CreatedAt,
			Language:    "en",
			Content:     richContent,
			SpoilerText: "",
			Visibility:  "public",
			Application: richActivityApplication{
				Name:    "xbsky.app",
				Website: "https://xbsky.app",
			},
			Account: richActivityAccount{
				ID:           strconv.Itoa(rand.Int()),
				DisplayName:  sortedAPI.StarterPack.Creator.DisplayName,
				UserName:     sortedAPI.StarterPack.Creator.Handle,
				Acct:         sortedAPI.StarterPack.Creator.Handle,
				URL:          "https://bsky.app/profile/" + sortedAPI.StarterPack.Creator.Handle,
				URI:          "https://bsky.app/profile/" + sortedAPI.StarterPack.Creator.Handle,
				Avatar:       sortedAPI.StarterPack.Creator.Avatar,
				AvatarStatic: sortedAPI.StarterPack.Creator.Avatar,
			},
			MediaAttachments: []richActivityMedia{},
		}

		ogCard := fmt.Sprintf("https://ogcard.cdn.bsky.app/start/%s/%s", sortedAPI.StarterPack.Creator.DID, actReqData.PostID)
		richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, richActivityMedia{
			ID:          strconv.Itoa(rand.Int()),
			Type:        "image",
			URL:         ogCard,
			Preview:     ogCard,
			Description: "",
		})
	default:
		errorPage(w, "Invalid type")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&richEmbed)
}

func genOembed(w http.ResponseWriter, r *http.Request) {
	media := r.URL.Query().Get("for")

	embed := oEmbed{
		Version:      "1.0",
		Type:         "link",
		ProviderName: "xbsky.app",
		ProviderURL:  "https://xbsky.app",
	}

	switch media {
	case "profile":
		followers, followersErr := strconv.ParseInt(r.URL.Query().Get("followers"), 10, 64)
		if followersErr != nil {
			http.Error(w, "genOembed: followers ParseInt failed", http.StatusInternalServerError)
			return
		}

		follows, followsErr := strconv.ParseInt(r.URL.Query().Get("follows"), 10, 64)
		if followsErr != nil {
			http.Error(w, "genOembed: follows ParseInt failed", http.StatusInternalServerError)
			return
		}

		posts, postsErr := strconv.ParseInt(r.URL.Query().Get("posts"), 10, 64)
		if postsErr != nil {
			http.Error(w, "genOembed: posts ParseInt failed", http.StatusInternalServerError)
			return
		}

		labeler, labelerErr := strconv.ParseBool(r.URL.Query().Get("labeler"))
		if labelerErr != nil {
			http.Error(w, "genOembed: labeler ParseBool failed", http.StatusInternalServerError)
			return
		}

		embed.AuthorName = fmt.Sprintf("👥 %s Followers - 🌐 %s Following - ✍️ %s Posts", toNotation(followers), toNotation(follows), toNotation(posts))

		if labeler {
			embed.AuthorName += " - 🏷️ Labeler"
		}
	case "post":
		replies, repliesErr := strconv.ParseInt(r.URL.Query().Get("replies"), 10, 64)
		if repliesErr != nil {
			http.Error(w, "genOembed: replies ParseInt failed", http.StatusInternalServerError)
			return
		}

		reposts, repostsErr := strconv.ParseInt(r.URL.Query().Get("reposts"), 10, 64)
		if repostsErr != nil {
			http.Error(w, "genOembed: reposts ParseInt failed", http.StatusInternalServerError)
			return
		}

		likes, likesErr := strconv.ParseInt(r.URL.Query().Get("likes"), 10, 64)
		if likesErr != nil {
			http.Error(w, "genOembed: likes ParseInt failed", http.StatusInternalServerError)
			return
		}

		quotes, quotesErr := strconv.ParseInt(r.URL.Query().Get("quotes"), 10, 64)
		if quotesErr != nil {
			http.Error(w, "genOembed: quotes ParseInt failed", http.StatusInternalServerError)
			return
		}

		embed.AuthorName = fmt.Sprintf("💬 %s   🔁 %s   🩷 %s   📝 %s", toNotation(replies), toNotation(reposts), toNotation(likes), toNotation(quotes))

		theDesc := r.URL.Query().Get("description")
		if theDesc != "" {
			var unescErr error

			theDesc, unescErr = url.PathUnescape(theDesc)
			if unescErr != nil {
				http.Error(w, "genOembed: description url.PathUnescape failed", http.StatusInternalServerError)
				return
			}

			cutLen := maxAuthorLen - len(embed.AuthorName+"\n\n")
			cutLen = max(cutLen, 0) // if cutLen < 0 {cutLen = 0}

			if len(theDesc) > cutLen {
				if cutLen >= ellipsisLen {
					theDesc = theDesc[:cutLen-ellipsisLen] + "..."
				} else {
					theDesc = theDesc[:cutLen]
				}
			}

			embed.AuthorName = embed.AuthorName + "\n\n" + theDesc
		}

		mediaMessage := r.URL.Query().Get("mediaMsg")
		if mediaMessage != "" {
			embed.ProviderName = fmt.Sprintf("%s | %s", embed.ProviderName, mediaMessage)
		}
	case "feed":
		likes, likesErr := strconv.ParseInt(r.URL.Query().Get("likes"), 10, 64)
		if likesErr != nil {
			http.Error(w, "genOembed: likes ParseInt failed", http.StatusInternalServerError)
			return
		}

		online, onlineErr := strconv.ParseBool(r.URL.Query().Get("online"))
		if onlineErr != nil {
			http.Error(w, "genOembed: online ParseBool failed", http.StatusInternalServerError)
			return
		}

		valid, validErr := strconv.ParseBool(r.URL.Query().Get("valid"))
		if validErr != nil {
			http.Error(w, "genOembed: valid ParseBool failed", http.StatusInternalServerError)
			return
		}

		embed.AuthorName = fmt.Sprintf("🩷 %s Likes", toNotation(likes))

		if online {
			embed.AuthorName += " - ✅ Online"
		} else {
			embed.AuthorName += " - ❌ Not online"
		}

		if valid {
			embed.AuthorName += " - ✅ Valid"
		} else {
			embed.AuthorName += " - ❌ Not valid"
		}
	default:
		http.Error(w, "genOembed: Invalid option", http.StatusInternalServerError)
		return
	}

	if encodeErr := json.NewEncoder(w).Encode(&embed); encodeErr != nil {
		http.Error(w, "genOembed: Failed to encode JSON", http.StatusInternalServerError)
		return
	}
}

func errorPage(w http.ResponseWriter, errorMessage string) {
	errorTemplate.Execute(w, map[string]string{"errorMsg": errorMessage})
}

func indexPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		errorPage(w, "route not found")
		return
	}

	http.Redirect(w, r, "https://github.com/colduw/xbsky", http.StatusFound)
}

func main() {
	sMux := http.NewServeMux()
	sMux.HandleFunc("GET /profile/{profileID}", getProfile)
	sMux.HandleFunc("GET /profile/{profileID}/post/{postID}", getPost)
	sMux.HandleFunc("GET /profile/{profileID}/post/{postID}/photo/{photoNum}", getPost)
	sMux.HandleFunc("GET /profile/{profileID}/feed/{feedID}", getFeed)
	sMux.HandleFunc("GET /profile/{profileID}/lists/{listID}", getList)
	sMux.HandleFunc("GET /starter-pack/{profileID}/{packID}", getPack)

	sMux.HandleFunc("GET /static/favicon.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./favicon.png")
	})

	sMux.HandleFunc("GET /users/{ignoredField}/statuses/{id}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://xbsky.app/api/v1/statuses/"+r.PathValue("id"), http.StatusFound)
	})

	sMux.HandleFunc("GET /api/v1/statuses/{id}", genActivity)
	sMux.HandleFunc("GET /oembed", genOembed)
	sMux.HandleFunc("GET /", indexPage)

	manager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist("xbsky.app", "raw.xbsky.app", "mosaic.xbsky.app", "api.xbsky.app"),
		Cache:      autocert.DirCache("certs"),
	}

	go blueskyHealthCheck()

	go func() {
		httpServer := &http.Server{
			Addr:              ":80",
			Handler:           manager.HTTPHandler(nil),
			ReadTimeout:       30 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       time.Minute,
		}

		if httpListenErr := httpServer.ListenAndServe(); httpListenErr != nil {
			panic(httpListenErr)
		}
	}()

	httpsServer := &http.Server{
		Addr:              ":443",
		Handler:           sMux,
		TLSConfig:         manager.TLSConfig(),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       time.Minute,
	}

	if httpsListenErr := httpsServer.ListenAndServeTLS("", ""); httpsListenErr != nil {
		panic(httpsListenErr)
	}
}

func toNotation(number int64) string {
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

func nl2br(in string) string {
	// This is escaped, but it somehow works.
	// I don't know, and I don't wanna know.
	return strings.ReplaceAll(in, "\n", "<br>")
}

// Check if bluesky is having issues (https://public.api.bsky.app/xrpc/_health)
// If this returns a non 200, it is most likely down (probably due to their ai slop usage)
// In that case, rewrite it to use the "private" api, which is the same, just w/o caching
// Not a guaranteed fix, since it works 50/50, but still better than the guaranteed down public api
func blueskyHealthCheck() {
	ticker := time.NewTicker(10 * time.Minute)

	for range ticker.C {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://public.api.bsky.app/xrpc/_health", http.NoBody)
		if err != nil {
			isBlueskyDead.Store(true)
			continue
		}

		resp, err := timeoutClient.Do(req)
		if err != nil {
			isBlueskyDead.Store(true)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			isBlueskyDead.Store(true)
			continue
		}

		resp.Body.Close()
		isBlueskyDead.Store(false)
	}
}

// https://www.agwa.name/blog/post/preventing_server_side_request_forgery_in_golang
func sDial(network, addr string, conn syscall.RawConn) error {
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
