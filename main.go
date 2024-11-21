package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

type (
	userProfile struct {
		Handle         string `json:"handle"`
		DisplayName    string `json:"displayName"`
		Avatar         string `json:"avatar"`
		Description    string `json:"description"`
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
			Creator     apiAuthor `json:"creator"`
		} `json:"list"`
	}

	apiPack struct {
		StarterPack struct {
			Record struct {
				Name        string `json:"name"`
				Description string `json:"description"`
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

	apiPost struct {
		Author apiAuthor `json:"author"`

		// Text of the post
		Record struct {
			Text      string `json:"text"`
			CreatedAt string `json:"createdAt"`
		} `json:"record"`

		// Embeds of stuff, if any.
		Embed struct {
			Type string `json:"$type"`

			// If this is a quote, and if there are embeds,
			// they'll be here
			Media mediaData `json:"media"`

			External struct {
				URI         string `json:"uri"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Thumb       string `json:"thumb"`
			} `json:"external"`

			// This is a text quote
			Record struct {
				// This is a quote with media
				Record struct {
					Value struct {
						Text string `json:"text"`
					} `json:"value"`

					Author apiAuthor `json:"author"`
				} `json:"record"`

				Value struct {
					Text string `json:"text"`
				} `json:"value"`

				Author apiAuthor `json:"author"`

				Embeds []struct {
					mediaData
					Media mediaData `json:"media"`
				} `json:"embeds"`
			} `json:"record"`

			Images apiImages `json:"images"`

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

		External struct {
			URI         string `json:"uri"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Thumb       string `json:"thumb"`
		} `json:"external"`

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

	// To reduce redundancy in the template
	ownData struct {
		Type string

		Author apiAuthor

		Record struct {
			Text      string `json:"text"`
			CreatedAt string `json:"createdAt"`
		}

		Images apiImages

		External struct {
			URI         string `json:"uri"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Thumb       string `json:"thumb"`
		}

		VideoCID string
		VideoDID string

		Description string
		StatsForTG  string

		Thumbnail   string
		AspectRatio apiAspectRatio

		ReplyCount  int64
		RepostCount int64
		LikeCount   int64
		QuoteCount  int64

		IsVideo bool
		IsGif   bool
	}
)

const (
	maxAuthorLen = 256
	ellipsisLen  = 3

	bskyEmbedImages   = "app.bsky.embed.images#view"
	bskyEmbedExternal = "app.bsky.embed.external#view"
	bskyEmbedVideo    = "app.bsky.embed.video#view"
	bskyEmbedQuote    = "app.bsky.embed.recordWithMedia#view"
	bskyEmbedText     = "app.bsky.embed.record#view"
	unknownType       = "unknownType"

	invalidHandle = "handle.invalid"

	modList    = "app.bsky.graph.defs#modlist"
	curateList = "app.bsky.graph.defs#curatelist"
)

var (
	timeoutClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	profileTemplate = template.Must(template.ParseFiles("./views/profile.html"))
	feedTemplate    = template.Must(template.ParseFiles("./views/feed.html"))
	listTemplate    = template.Must(template.ParseFiles("./views/list.html"))
	packTemplate    = template.Must(template.ParseFiles("./views/pack.html"))
	postTemplate    = template.Must(template.New("post.html").Funcs(template.FuncMap{"escapePath": url.PathEscape}).ParseFiles("./views/post.html"))
	errorTemplate   = template.Must(template.ParseFiles("./views/error.html"))
)

func resolveHandle(ctx context.Context, handle string) string {
	apiURL := "https://public.api.bsky.app/xrpc/com.atproto.identity.resolveHandle?handle=" + handle

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		return handle
	}

	resp, respErr := timeoutClient.Do(req)
	if respErr != nil {
		return handle
	}

	//nolint:errcheck // this should not fail, but even if it did, at most, we'd just log that it failed
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return handle
	}

	var uDID apiDID
	if decodeErr := json.NewDecoder(resp.Body).Decode(&uDID); decodeErr != nil {
		return handle
	}

	if !strings.HasPrefix(uDID.DID, "did:plc") {
		return handle
	}

	return uDID.DID
}

func getProfile(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	profileID = strings.ReplaceAll(profileID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = resolveHandle(r.Context(), editedPID)
	}

	apiURL := "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=" + editedPID

	req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		errorPage(w, "getProfile: Failed to create request")
		return
	}

	resp, respErr := timeoutClient.Do(req)
	if respErr != nil {
		errorPage(w, "getProfile: Failed to do request")
		return
	}

	//nolint:errcheck // this should not fail, but even if it did, at most, we'd just log that it failed
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

	if profile.Handle == invalidHandle {
		profile.Handle = profileID
	}

	isTelegramAgent := strings.Contains(r.Header.Get("User-Agent"), "Telegram")

	if execErr := profileTemplate.Execute(w, map[string]any{"profile": profile, "isTelegram": isTelegramAgent}); execErr != nil {
		http.Error(w, "getProfile: Failed to execute template", http.StatusInternalServerError)
		return
	}
}

func getFeed(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	feedID := r.PathValue("feedID")
	feedID = strings.ReplaceAll(feedID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = resolveHandle(r.Context(), editedPID)
	}

	if !strings.HasPrefix(editedPID, "at://") {
		editedPID = "at://" + editedPID
	}

	apiURL := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.feed.getFeedGenerator?feed=%s/app.bsky.feed.generator/%s", editedPID, feedID)

	req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		errorPage(w, "getFeed: failed to create request")
		return
	}

	resp, respErr := timeoutClient.Do(req)
	if respErr != nil {
		errorPage(w, "getFeed: failed to do request")
		return
	}

	//nolint:errcheck // this should not fail, but even if it did, at most, we'd just log that it failed
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorPage(w, fmt.Sprintf("getFeed: Unexpected status (%s)", resp.Status))
		return
	}

	var feed apiFeed
	if decodeErr := json.NewDecoder(resp.Body).Decode(&feed); decodeErr != nil {
		errorPage(w, "getFeed: failed to decode response")
	}

	if feed.View.Creator.Handle == invalidHandle {
		feed.View.Creator.Handle = profileID
	}

	feed.View.Description = fmt.Sprintf("📡 A feed by %s (@%s)\n\n", feed.View.Creator.DisplayName, feed.View.Creator.Handle) + feed.View.Description

	isTelegramAgent := strings.Contains(r.Header.Get("User-Agent"), "Telegram")

	if execErr := feedTemplate.Execute(w, map[string]any{"feed": feed, "feedID": feedID, "isTelegram": isTelegramAgent}); execErr != nil {
		http.Error(w, "getFeed: failed to execute template", http.StatusInternalServerError)
		return
	}
}

func getList(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	listID := r.PathValue("listID")
	listID = strings.ReplaceAll(listID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = resolveHandle(r.Context(), editedPID)
	}

	if !strings.HasPrefix(editedPID, "at://") {
		editedPID = "at://" + editedPID
	}

	apiURL := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.graph.getList?limit=1&list=%s/app.bsky.graph.list/%s", editedPID, listID)

	req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		errorPage(w, "getList: failed to create request")
		return
	}

	resp, respErr := timeoutClient.Do(req)
	if respErr != nil {
		errorPage(w, "getList: failed to do request")
		return
	}

	//nolint:errcheck // this should not fail, but even if it did, at most, we'd just log that it failed
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorPage(w, fmt.Sprintf("getList: Unexpected status (%s)", resp.Status))
		return
	}

	var list apiList
	if decodeErr := json.NewDecoder(resp.Body).Decode(&list); decodeErr != nil {
		errorPage(w, "getList: failed to decode response")
	}

	if list.List.Creator.Handle == invalidHandle {
		list.List.Creator.Handle = profileID
	}

	switch list.List.Purpose {
	case modList:
		list.List.Description = fmt.Sprintf("🚫 A moderation list by %s (@%s)\n\n", list.List.Creator.DisplayName, list.List.Creator.Handle) + list.List.Description
	case curateList:
		list.List.Description = fmt.Sprintf("👥 A curator list by %s (@%s)\n\n", list.List.Creator.DisplayName, list.List.Creator.Handle) + list.List.Description
	}

	isTelegramAgent := strings.Contains(r.Header.Get("User-Agent"), "Telegram")

	if execErr := listTemplate.Execute(w, map[string]any{"list": list.List, "listID": listID, "isTelegram": isTelegramAgent}); execErr != nil {
		http.Error(w, "getList: failed to execute template", http.StatusInternalServerError)
		return
	}
}

func getPack(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	packID := r.PathValue("packID")
	packID = strings.ReplaceAll(packID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = resolveHandle(r.Context(), editedPID)
	}

	if !strings.HasPrefix(editedPID, "at://") {
		editedPID = "at://" + editedPID
	}

	apiURL := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.graph.getStarterPack?starterPack=%s/app.bsky.graph.starterpack/%s", editedPID, packID)

	req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		errorPage(w, "getPack: failed to create request")
		return
	}

	resp, respErr := timeoutClient.Do(req)
	if respErr != nil {
		errorPage(w, "getPack: failed to do request")
		return
	}

	//nolint:errcheck // this should not fail, but even if it did, at most, we'd just log that it failed
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errorPage(w, fmt.Sprintf("getPack: Unexpected status (%s)", resp.Status))
		return
	}

	var pack apiPack
	if decodeErr := json.NewDecoder(resp.Body).Decode(&pack); decodeErr != nil {
		errorPage(w, "getPack: failed to decode response")
	}

	if pack.StarterPack.Creator.Handle == invalidHandle {
		pack.StarterPack.Creator.Handle = profileID
	}

	pack.StarterPack.Record.Description = fmt.Sprintf("📦 A starter pack by %s (@%s)", pack.StarterPack.Creator.DisplayName, pack.StarterPack.Creator.Handle) + pack.StarterPack.Record.Description

	isTelegramAgent := strings.Contains(r.Header.Get("User-Agent"), "Telegram")

	if execErr := packTemplate.Execute(w, map[string]any{"pack": pack.StarterPack, "packID": packID, "isTelegram": isTelegramAgent}); execErr != nil {
		http.Error(w, "getPack: failed to execute template", http.StatusInternalServerError)
		return
	}
}

func getPost(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	postID := r.PathValue("postID")
	postID = strings.ReplaceAll(postID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = resolveHandle(r.Context(), editedPID)
	}

	if !strings.HasPrefix(editedPID, "at://") {
		editedPID = "at://" + editedPID
	}

	postAPIURL := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.feed.getPostThread?depth=0&uri=%s/app.bsky.feed.post/%s", editedPID, postID)

	postReq, postReqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, postAPIURL, http.NoBody)
	if postReqErr != nil {
		errorPage(w, "getPost: Failed to create request")
		return
	}

	postResp, postRespErr := timeoutClient.Do(postReq)
	if postRespErr != nil {
		errorPage(w, "getPost: Failed to do request")
		return
	}

	//nolint:errcheck // this should not fail, but even if it did, at most, we'd just log that it failed
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
	if selfData.Author.Handle == invalidHandle {
		selfData.Author.Handle = profileID
	}

	selfData.Record = postData.Thread.Post.Record

	selfData.ReplyCount = postData.Thread.Post.ReplyCount
	selfData.RepostCount = postData.Thread.Post.RepostCount
	selfData.LikeCount = postData.Thread.Post.LikeCount
	selfData.QuoteCount = postData.Thread.Post.QuoteCount

	selfData.Description = selfData.Record.Text
	selfData.StatsForTG = fmt.Sprintf("💬 %d   🔁 %d   ❤️ %d   📝 %d", postData.Thread.Post.ReplyCount, postData.Thread.Post.RepostCount, postData.Thread.Post.LikeCount, postData.Thread.Post.QuoteCount)

	// This is to reduce redundancy in the templates
	switch postData.Thread.Post.Embed.Type {
	case bskyEmbedImages:
		// Image(s)
		selfData.Type = bskyEmbedImages
		selfData.Images = postData.Thread.Post.Embed.Images
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
		case bskyEmbedExternal:
			selfData.Type = bskyEmbedExternal
			selfData.External = postData.Thread.Post.Embed.Media.External
		case bskyEmbedVideo:
			selfData.Type = bskyEmbedVideo
			selfData.VideoCID = postData.Thread.Post.Embed.Media.CID
			selfData.VideoDID = postData.Thread.Post.Embed.Record.Record.Author.DID
			selfData.AspectRatio = postData.Thread.Post.Embed.Media.AspectRatio
			selfData.Thumbnail = postData.Thread.Post.Embed.Media.Thumbnail
			selfData.IsVideo = true
		default:
			selfData.Type = unknownType
		}
	default:
		// Text post, check if parent or quote
		if postData.Thread.Parent != nil {
			// Reply
			switch postData.Thread.Parent.Post.Embed.Type {
			case bskyEmbedImages:
				selfData.Type = bskyEmbedImages
				selfData.Images = postData.Thread.Parent.Post.Embed.Images
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
				case bskyEmbedExternal:
					selfData.Type = bskyEmbedExternal
					selfData.External = postData.Thread.Parent.Post.Embed.Media.External
				case bskyEmbedVideo:
					selfData.Type = bskyEmbedVideo
					selfData.VideoCID = postData.Thread.Parent.Post.Embed.Media.CID
					selfData.VideoDID = postData.Thread.Parent.Post.Embed.Record.Record.Author.DID
					selfData.AspectRatio = postData.Thread.Parent.Post.Embed.Media.AspectRatio
					selfData.Thumbnail = postData.Thread.Parent.Post.Embed.Media.Thumbnail
					selfData.IsVideo = true
				default:
					selfData.Type = unknownType
				}
			default:
				selfData.Type = unknownType
			}
		} else if postData.Thread.Post.Embed.Type == bskyEmbedText {
			// Do we have any quote embeds?
			if len(postData.Thread.Post.Embed.Record.Embeds) > 0 {
				// Yup
				theEmbed := postData.Thread.Post.Embed.Record.Embeds[0]

				switch theEmbed.Type {
				case bskyEmbedImages:
					selfData.Type = bskyEmbedImages
					selfData.Images = theEmbed.Images
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
					selfData.Type = unknownType
				}
			} else {
				// Nope
				selfData.Type = unknownType
			}
		} else {
			selfData.Type = unknownType
		}
	}

	// Check the external; is it a GIF?
	if selfData.Type == bskyEmbedExternal {
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
	}

	if strings.HasPrefix(r.Host, "mosaic.") {
		if selfData.Type == bskyEmbedImages {
			genMosaic(w, r, selfData.Images)
			return
		}

		errorPage(w, "getPost: Invalid type")
		return
	}

	if strings.HasPrefix(r.Host, "raw.") {
		switch selfData.Type {
		case bskyEmbedImages:
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
			http.Redirect(w, r, fmt.Sprintf("https://bsky.social/xrpc/com.atproto.sync.getBlob?cid=%s&did=%s", selfData.VideoCID, selfData.VideoDID), http.StatusFound)
			return
		default:
			errorPage(w, "getPost: Invalid type")
			return
		}
	}

	// This is just so I won't have to look for it
	if postData.Thread.Parent != nil {
		selfData.Description += fmt.Sprintf("\n\n💬 Replying to %s (@%s):\n\n%s", postData.Thread.Parent.Post.Author.DisplayName, postData.Thread.Parent.Post.Author.Handle, postData.Thread.Parent.Post.Record.Text)
	}

	switch postData.Thread.Post.Embed.Type {
	case bskyEmbedText:
		selfData.Description += fmt.Sprintf("\n\n📝 Quoting %s (@%s):\n\n%s", postData.Thread.Post.Embed.Record.Author.DisplayName, postData.Thread.Post.Embed.Record.Author.Handle, postData.Thread.Post.Embed.Record.Value.Text)
	case bskyEmbedQuote:
		selfData.Description += fmt.Sprintf("\n\n📝 Quoting %s (@%s):\n\n%s", postData.Thread.Post.Embed.Record.Record.Author.DisplayName, postData.Thread.Post.Embed.Record.Record.Author.Handle, postData.Thread.Post.Embed.Record.Record.Value.Text)
	}

	isTelegramAgent := strings.Contains(r.Header.Get("User-Agent"), "Telegram")

	if execErr := postTemplate.Execute(w, map[string]any{"data": selfData, "editedPID": strings.TrimPrefix(editedPID, "at://"), "postID": postID, "isTelegram": isTelegramAgent}); execErr != nil {
		http.Error(w, "getPost: Failed to execute template", http.StatusInternalServerError)
		return
	}
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

	w.Header().Set("Content-Type", "image/png")

	//nolint:prealloc // No
	var args []string
	for _, k := range images {
		args = append(args, "-i", k.FullSize)
	}

	var filterComplex string
	for i := range images {
		filterComplex += fmt.Sprintf("[%d:v]scale=-1:600[m%d];", i, i)
	}

	for i := range images {
		filterComplex += fmt.Sprintf("[m%d]", i)
	}
	filterComplex += fmt.Sprintf("hstack=inputs=%d", len(images))

	args = append(args, "-filter_complex", filterComplex, "-f", "image2pipe", "-c:v", "png", "pipe:1")

	//nolint:gosec // This is just ffmpeg, with the only external values being k.FullSize, which is from the API
	cmd := exec.CommandContext(r.Context(), "ffmpeg", args...)
	cmd.Stdout = w

	if runErr := cmd.Run(); runErr != nil {
		http.Error(w, "genMosaic: Failed to run", http.StatusInternalServerError)
		return
	}
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

		embed.AuthorName = fmt.Sprintf("👥 %d Followers - 🌐 %d Following - ✍️ %d Posts", followers, follows, posts)

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

		embed.AuthorName = fmt.Sprintf("💬 %d   🔁 %d   ❤️ %d   📝 %d", replies, reposts, likes, quotes)

		theDesc := r.URL.Query().Get("description")
		if theDesc != "" {
			var unescErr error

			theDesc, unescErr = url.PathUnescape(theDesc)
			if unescErr != nil {
				http.Error(w, "genOembed: description url.PathUnescape failed", http.StatusInternalServerError)
				return
			}

			cutLen := maxAuthorLen - len(embed.AuthorName+"\n\n")

			if cutLen < 0 {
				cutLen = 0
			}

			if len(theDesc) > cutLen {
				if cutLen >= ellipsisLen {
					theDesc = theDesc[:cutLen-ellipsisLen] + "..."
				} else {
					theDesc = theDesc[:cutLen]
				}
			}

			embed.AuthorName = embed.AuthorName + "\n\n" + theDesc
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

		embed.AuthorName = fmt.Sprintf("❤️ %d Likes", likes)

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
	if execErr := errorTemplate.Execute(w, map[string]string{"errorMsg": errorMessage}); execErr != nil {
		http.Error(w, "Failed to execute template", http.StatusInternalServerError)
		return
	}
}

func redirToGithub(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://github.com/colduw/xbsky", http.StatusFound)
}

func main() {
	sMux := http.NewServeMux()
	sMux.HandleFunc("GET /profile/{profileID}", getProfile)
	sMux.HandleFunc("GET /profile/{profileID}/post/{postID}", getPost)
	sMux.HandleFunc("GET /profile/{profileID}/feed/{feedID}", getFeed)
	sMux.HandleFunc("GET /profile/{profileID}/lists/{listID}", getList)
	sMux.HandleFunc("GET /starter-pack/{profileID}/{packID}", getPack)
	sMux.HandleFunc("GET /oembed", genOembed)
	sMux.HandleFunc("GET /", redirToGithub)

	manager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist("xbsky.app", "raw.xbsky.app", "mosaic.xbsky.app"),
		Cache:      autocert.DirCache("certs/"),
	}

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
