package handlers

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"main/internal/helpers"
	"main/internal/types"
)

var postTemplate = template.Must(template.New("post.html").Funcs(template.FuncMap{"escapePath": url.PathEscape, "nl2br": helpers.NL2BR}).ParseFiles("./views/post.html"))

func (ps *HandlerPass) GetPost(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	postID := r.PathValue("postID")
	postID = strings.ReplaceAll(postID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = helpers.ResolveHandle(r.Context(), editedPID)
	}
	plcData := helpers.ResolvePLC(r.Context(), editedPID)

	if !strings.HasPrefix(editedPID, "at://") {
		editedPID = "at://" + editedPID
	}

	apiURL := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.feed.getPostThread?depth=0&uri=%s/app.bsky.feed.post/%s", editedPID, postID)
	if helpers.IsBlueskyDead.Load() {
		apiURL = fmt.Sprintf("https://api.bsky.app/xrpc/app.bsky.feed.getPostThread?depth=0&uri=%s/app.bsky.feed.post/%s", editedPID, postID)
	}

	postReq, postReqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if postReqErr != nil {
		ErrorPage(w, "getPost: Failed to create request")
		return
	}

	postResp, postRespErr := helpers.TimeoutClient.Do(postReq)
	if errors.Is(postRespErr, context.DeadlineExceeded) {
		ErrorPage(w, "getPost: Bluesky took too long to respond (timeout exceeded)")
		return
	} else if postRespErr != nil {
		ErrorPage(w, "getPost: Failed to do request")
		return
	}

	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK {
		ErrorPage(w, fmt.Sprintf("getPost: Unexpected status (%s)", postResp.Status))
		return
	}

	var postData types.APIThread

	if decodeErr := json.NewDecoder(postResp.Body).Decode(&postData); decodeErr != nil {
		ErrorPage(w, "getPost: Failed to decode response")
		return
	}

	// Build data here instead of in the template
	var selfData types.OwnData

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
	selfData.StatsForTG = fmt.Sprintf("💬 %s   🔁 %s   🩷 %s   📝 %s", helpers.ToNotation(postData.Thread.Post.ReplyCount), helpers.ToNotation(postData.Thread.Post.RepostCount), helpers.ToNotation(postData.Thread.Post.LikeCount), helpers.ToNotation(postData.Thread.Post.QuoteCount))

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
			selfData.IsGif = (parsedURL.Host == "media.tenor.com" || parsedURL.Host == "static.klipy.com")
		}

		if selfData.IsGif {
			// The template is stupidly persistent on rewriting & to &amp; come hell or high water it will rewrite it
			selfData.External.URI = "https://" + parsedURL.Host + parsedURL.Path
		} else {
			// Not a GIF, Add the external's title & description to the template description
			selfData.Description += "\n\n" + selfData.External.Title + "\n" + selfData.External.Description
		}
	case bskyEmbedImages, galleryImages:
		pnStr := r.PathValue("photoNum")
		if pnStr != "" {
			pnValue, atoiErr := strconv.Atoi(pnStr)
			if atoiErr != nil {
				ErrorPage(w, "getPost: Invalid photo number")
				return
			}

			if pnValue < 1 {
				pnValue = 1
			}

			imgLen := len(selfData.Images)
			if imgLen > 1 && imgLen >= pnValue {
				mediaMsg = fmt.Sprintf("Photo %d of %d", pnValue, imgLen)
				selfData.Images = types.APIImages{selfData.Images[pnValue-1]}
			}
		}
	case bskyEmbedVideo:
		vidOwnerPLC := helpers.ResolvePLC(r.Context(), selfData.VideoDID)
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
			GenMosaic(w, r, selfData.Images)
			return
		}

		ErrorPage(w, "getPost: Invalid type")
		return
	}

	if strings.HasPrefix(r.Host, "raw.") {
		switch selfData.Type {
		case bskyEmbedImages, galleryImages:
			GenMosaic(w, r, selfData.Images)
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

			ErrorPage(w, "getPost: No suitable media found")
			return
		case bskyEmbedVideo:
			http.Redirect(w, r, fmt.Sprintf("%s/xrpc/com.atproto.sync.getBlob?cid=%s&did=%s", selfData.PDS, selfData.VideoCID, selfData.VideoDID), http.StatusFound)
			return
		case bskyEmbedList, bskyEmbedPack, bskyEmbedFeed:
			if selfData.CommonEmbeds.Avatar != "" {
				http.Redirect(w, r, selfData.CommonEmbeds.Avatar, http.StatusFound)
				return
			}

			ErrorPage(w, "getPost: No suitable media found")
			return
		default:
			ErrorPage(w, "getPost: Invalid type")
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

	encodedID := types.RichActivityEncoded{
		Type:     "post",
		Handle:   selfData.Author.DID,
		PostID:   postID,
		PhotoCut: r.PathValue("photoNum"),
	}

	marshaled, err := json.Marshal(encodedID)
	if err != nil {
		ErrorPage(w, "getPost: failed to marshal for activity")
		return
	}

	postTemplate.Execute(w, map[string]any{"data": selfData, "editedPID": strings.TrimPrefix(editedPID, "at://"), "postID": postID, "isTelegram": isTelegramAgent, "mediaMsg": mediaMsg, "encodedID": hex.EncodeToString(marshaled), "passData": ps})
}
