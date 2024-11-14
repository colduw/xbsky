package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
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

	apiPost struct {
		Author struct {
			DID         string `json:"did"`
			Handle      string `json:"handle"`
			DisplayName string `json:"displayName"`
			Avatar      string `json:"avatar"`
		} `json:"author"`

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
				Description string `json:"description"`
				URI         string `json:"uri"`
			} `json:"external"`

			// This is a text quote
			Record struct {
				// This is a quote with media
				Record struct {
					Value struct {
						Text string `json:"text"`
					} `json:"value"`

					Author struct {
						DID         string `json:"did"`
						Handle      string `json:"handle"`
						DisplayName string `json:"displayName"`
					} `json:"author"`
				} `json:"record"`

				Value struct {
					Text string `json:"text"`
				} `json:"value"`

				Author struct {
					DID         string `json:"did"`
					Handle      string `json:"handle"`
					DisplayName string `json:"displayName"`
				} `json:"author"`

				Embeds []struct {
					mediaData
					Media mediaData `json:"media"`
				} `json:"embeds"`
			} `json:"record"`

			Images []struct {
				FullSize    string         `json:"fullsize"`
				Alt         string         `json:"alt"`
				AspectRatio apiAspectRatio `json:"aspectRatio"`
			} `json:"images"`

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

		Images []struct {
			FullSize    string         `json:"fullsize"`
			Alt         string         `json:"alt"`
			AspectRatio apiAspectRatio `json:"aspectRatio"`
		} `json:"images"`

		External struct {
			Description string `json:"description"`
			URI         string `json:"uri"`
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

		Author struct {
			DID         string `json:"did"`
			Handle      string `json:"handle"`
			DisplayName string `json:"displayName"`
			Avatar      string `json:"avatar"`
		}

		Record struct {
			Text      string `json:"text"`
			CreatedAt string `json:"createdAt"`
		}

		Images []struct {
			FullSize    string         `json:"fullsize"`
			Alt         string         `json:"alt"`
			AspectRatio apiAspectRatio `json:"aspectRatio"`
		}

		External struct {
			Description string `json:"description"`
			URI         string `json:"uri"`
		}

		VideoCID string
		VideoDID string

		AddnDesc   string
		StatsForTG string

		Thumbnail   string
		AspectRatio apiAspectRatio

		ReplyCount  int64
		RepostCount int64
		LikeCount   int64
		QuoteCount  int64

		IsVideo bool
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
)

var (
	timeoutClient = &http.Client{
		Timeout: time.Minute,
	}

	profileTemplate = template.Must(template.ParseFiles("./views/profile.html"))
	postTemplate    = template.Must(template.New("post.html").Funcs(template.FuncMap{"escapePath": url.PathEscape}).ParseFiles("./views/post.html"))
	errorTemplate   = template.Must(template.ParseFiles("./views/error.html"))
)

func getProfile(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")

	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()

	apiURL := "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=" + profileID

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, http.NoBody)
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

	if execErr := profileTemplate.Execute(w, map[string]userProfile{"profile": profile}); execErr != nil {
		http.Error(w, "getProfile: Failed to execute template", http.StatusInternalServerError)
		return
	}
}

func getPost(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	postID := r.PathValue("postID")
	postID = strings.ReplaceAll(postID, "|", "")

	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()

	if !strings.HasPrefix(profileID, "at://") {
		profileID = "at://" + profileID
	}

	postAPIURL := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.feed.getPostThread?uri=%s/app.bsky.feed.post/%s", profileID, postID)

	postReq, postReqErr := http.NewRequestWithContext(ctx, http.MethodGet, postAPIURL, http.NoBody)
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
	selfData.Record = postData.Thread.Post.Record

	selfData.ReplyCount = postData.Thread.Post.ReplyCount
	selfData.RepostCount = postData.Thread.Post.RepostCount
	selfData.LikeCount = postData.Thread.Post.LikeCount
	selfData.QuoteCount = postData.Thread.Post.QuoteCount

	selfData.StatsForTG = fmt.Sprintf("💬 %d   🔁 %d   ❤️ %d   📝 %d", postData.Thread.Post.ReplyCount, postData.Thread.Post.RepostCount, postData.Thread.Post.LikeCount, postData.Thread.Post.QuoteCount)

	// This is just so I won't have to look for it
	if postData.Thread.Parent != nil {
		selfData.AddnDesc = fmt.Sprintf("💬 Replying to %s (@%s):\n\n%s", postData.Thread.Parent.Post.Author.DisplayName, postData.Thread.Parent.Post.Author.Handle, postData.Thread.Parent.Post.Record.Text)
	}

	switch postData.Thread.Post.Embed.Type {
	case bskyEmbedText:
		selfData.AddnDesc = fmt.Sprintf("📝 Quoting %s (@%s):\n\n%s", postData.Thread.Post.Embed.Record.Author.DisplayName, postData.Thread.Post.Embed.Record.Author.Handle, postData.Thread.Post.Embed.Record.Value.Text)
	case bskyEmbedQuote:
		selfData.AddnDesc = fmt.Sprintf("📝 Quoting %s (@%s):\n\n%s", postData.Thread.Post.Embed.Record.Record.Author.DisplayName, postData.Thread.Post.Embed.Record.Record.Author.Handle, postData.Thread.Post.Embed.Record.Record.Value.Text)
	}

	// This is to reduce redundancy in the templates
	switch postData.Thread.Post.Embed.Type {
	case bskyEmbedImages:
		// Image(s)
		selfData.Type = bskyEmbedImages
		selfData.Images = postData.Thread.Post.Embed.Images
	case bskyEmbedExternal:
		// External (eg gifs)
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

	if strings.HasPrefix(r.Host, "raw.") {
		switch selfData.Type {
		case bskyEmbedImages:
			// TODO: create horizontally stacked image with ffmpeg -filter_complex hstack, and resize to -1:600
			// on another route or sub
			if len(selfData.Images) > 0 {
				http.Redirect(w, r, selfData.Images[0].FullSize, http.StatusFound)
				return
			}

			return
		case bskyEmbedExternal:
			http.Redirect(w, r, selfData.External.URI, http.StatusFound)
			return
		case bskyEmbedVideo:
			http.Redirect(w, r, fmt.Sprintf("https://bsky.social/xrpc/com.atproto.sync.getBlob?cid=%s&did=%s", selfData.VideoCID, selfData.VideoDID), http.StatusFound)
			return
		default:
			errorPage(w, "getPost: Invalid type")
			return
		}
	}

	isDiscordAgent := strings.Contains(r.Header.Get("User-Agent"), "Discord")
	isTelegramAgent := strings.Contains(r.Header.Get("User-Agent"), "Telegram")

	if execErr := postTemplate.Execute(w, map[string]any{"data": selfData, "postID": postID, "isDiscord": isDiscordAgent, "isTelegram": isTelegramAgent}); execErr != nil {
		http.Error(w, "getPost: Failed to execute template", http.StatusInternalServerError)
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

		embed.AuthorName = fmt.Sprintf("👥 %d Followers - 🌐 %d Following - ✍️ %d Posts", followers, follows, posts)
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

		postDesc := r.URL.Query().Get("description")
		additionalDesc := r.URL.Query().Get("addndesc")

		theDesc := postDesc + additionalDesc
		if theDesc != "" {
			theDesc = postDesc + "\n\n" + additionalDesc

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
	sMux.HandleFunc("GET /oembed", genOembed)
	sMux.HandleFunc("GET /", redirToGithub)

	manager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist("xbsky.app", "raw.xbsky.app"),
		Cache:      autocert.DirCache("certs/"),
	}

	go func() {
		httpServer := &http.Server{
			Addr:              ":80",
			Handler:           manager.HTTPHandler(nil),
			ReadTimeout:       20 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      20 * time.Second,
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
		ReadTimeout:       20 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       time.Minute,
	}

	if httpsListenErr := httpsServer.ListenAndServeTLS("", ""); httpsListenErr != nil {
		panic(httpsListenErr)
	}
}
