package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
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
			Text string `json:"text"`
		} `json:"record"`

		// Embeds of stuff, if any.
		Embed struct {
			Type string `json:"$type"`

			// If this is a quote, and if there are embeds,
			// they'll be here
			Media struct {
				Type string `json:"$type"`

				Images []struct {
					FullSize    string         `json:"fullsize"`
					AspectRatio apiAspectRatio `json:"aspectRatio"`
				} `json:"images"`

				External struct {
					URI string `json:"uri"`
				} `json:"external"`

				CID         string         `json:"cid"`
				Thumbnail   string         `json:"thumbnail"`
				AspectRatio apiAspectRatio `json:"aspectRatio"`
			} `json:"media"`

			External struct {
				URI string `json:"uri"`
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
					Handle      string `json:"handle"`
					DisplayName string `json:"displayName"`
				} `json:"author"`
			} `json:"record"`

			Images []struct {
				FullSize    string         `json:"fullsize"`
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
)

const (
	maxAuthorLen = 256
	ellipsisLen  = 3
)

var (
	timeoutClient = &http.Client{
		Timeout: time.Minute,
	}

	profileTemplate = template.Must(template.ParseFiles("./views/profile.html"))
	postTemplate    = template.Must(template.ParseFiles("./views/post.html"))
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

	if strings.HasPrefix(r.Host, "raw.") {
		// Essentially post.html L33-L89
		switch postData.Thread.Post.Embed.Type {
		case "app.bsky.embed.images#view":
			// TODO: One big image
			if len(postData.Thread.Post.Embed.Images) > 0 {
				// For now, redirect to the first one
				http.Redirect(w, r, postData.Thread.Post.Embed.Images[0].FullSize, http.StatusFound)
				return
			}
		case "app.bsky.embed.external#view":
			http.Redirect(w, r, postData.Thread.Post.Embed.External.URI, http.StatusFound)
			return
		case "app.bsky.embed.video#view":
			http.Redirect(w, r, fmt.Sprintf("https://bsky.social/xrpc/com.atproto.sync.getBlob?cid=%s&did=%s", postData.Thread.Post.Embed.CID, postData.Thread.Post.Author.DID), http.StatusFound)
			return
		case "app.bsky.embed.recordWithMedia#view":
			switch postData.Thread.Post.Embed.Media.Type {
			case "app.bsky.embed.images#view":
				if len(postData.Thread.Post.Embed.Media.Images) > 0 {
					http.Redirect(w, r, postData.Thread.Post.Embed.Media.Images[0].FullSize, http.StatusFound)
					return
				}
			case "app.bsky.embed.external#view":
				http.Redirect(w, r, postData.Thread.Post.Embed.Media.External.URI, http.StatusFound)
				return
			case "app.bsky.embed.video#view":
				http.Redirect(w, r, fmt.Sprintf("https://bsky.social/xrpc/com.atproto.sync.getBlob?cid=%s&did=%s", postData.Thread.Post.Embed.Media.CID, postData.Thread.Post.Embed.Record.Record.Author.DID), http.StatusFound)
				return
			}
		}
	}

	postData.Thread.Post.Record.Text = strings.ReplaceAll(postData.Thread.Post.Record.Text, "|", "")

	if execErr := postTemplate.Execute(w, map[string]any{"data": postData, "postID": postID}); execErr != nil {
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

		embed.AuthorName = fmt.Sprintf("💬 %d Replies - 🔁 %d Reposts - ❤️ %d Likes - 📝 %d Quotes", replies, reposts, likes, quotes)

		theDesc := r.URL.Query().Get("description")
		if theDesc != "" {
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
