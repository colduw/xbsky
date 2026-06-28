package handlers

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"

	"main/internal/helpers"
	"main/internal/types"
)

// source: https://compiles.me/blog/making-rich-url-embeds-for-discord && https://embedl.ink/
// thanks!
func (ps *HandlerPass) GenActivity(w http.ResponseWriter, r *http.Request) {
	encodedID := r.PathValue("id")

	hBytes, err := hex.DecodeString(encodedID)
	if err != nil {
		ErrorPage(w, "invalid ID")
		return
	}

	var actReqData types.RichActivityEncoded
	if unmarshalErr := json.Unmarshal(hBytes, &actReqData); unmarshalErr != nil {
		ErrorPage(w, "failed to unmarshal JSON")
		return
	}

	var richEmbed types.RichActivity
	switch actReqData.Type {
	case "post":
		apiURL := fmt.Sprintf("https://api.%s/profile/%s/post/%s", ps.DomainName, actReqData.Handle, actReqData.PostID)
		if actReqData.PhotoCut != "" {
			apiURL = fmt.Sprintf("https://api.%s/profile/%s/post/%s/photo/%s", ps.DomainName, actReqData.Handle, actReqData.PostID, actReqData.PhotoCut)
		}

		newAPIReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
		if err != nil {
			ErrorPage(w, "failed to request api data")
			return
		}

		apiResp, err := helpers.TimeoutClient.Do(newAPIReq)
		if err != nil {
			ErrorPage(w, "failed to do api request")
			return
		}

		defer apiResp.Body.Close()

		var sortedAPI types.SortedAPIResponse

		if decodeErr := json.NewDecoder(apiResp.Body).Decode(&sortedAPI); decodeErr != nil {
			ErrorPage(w, "failed to decode response")
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

		if sortedAPI.ParsedData.External.Title != "" && !sortedAPI.ParsedData.IsGif {
			richContent += fmt.Sprintf(`<blockquote><p>%s</p><p>%s</p><p><a href=%q>%s</a></p></blockquote>`, sortedAPI.ParsedData.External.Title, sortedAPI.ParsedData.External.Description, sortedAPI.ParsedData.External.URI, sortedAPI.ParsedData.External.URI)
		}

		richContent = strings.ReplaceAll(richContent, "\n", "<br>")

		richContent += fmt.Sprintf("<p>%s</p>", fmt.Sprintf("💬 %s &ensp; 🔁 %s &ensp; 🩷 %s &ensp; 📝 %s", helpers.ToNotation(sortedAPI.ParsedData.ReplyCount), helpers.ToNotation(sortedAPI.ParsedData.RepostCount), helpers.ToNotation(sortedAPI.ParsedData.LikeCount), helpers.ToNotation(sortedAPI.ParsedData.QuoteCount)))

		richEmbed = types.RichActivity{
			ID:          strconv.Itoa(rand.Int()),
			URL:         fmt.Sprintf("https://bsky.app/profile/%s/post/%s", actReqData.Handle, actReqData.PostID),
			URI:         fmt.Sprintf("https://bsky.app/profile/%s/post/%s", actReqData.Handle, actReqData.PostID),
			CreatedAt:   sortedAPI.ParsedData.Record.CreatedAt,
			Language:    "en",
			Content:     richContent,
			SpoilerText: "",
			Visibility:  "public",
			Application: types.RichActivityApplication{
				Name:    ps.DomainName,
				Website: "https://" + ps.DomainName,
			},
			Account: types.RichActivityAccount{
				ID:           strconv.Itoa(rand.Int()),
				DisplayName:  sortedAPI.ParsedData.Author.DisplayName,
				UserName:     sortedAPI.ParsedData.Author.Handle,
				Acct:         sortedAPI.ParsedData.Author.Handle,
				URL:          "https://bsky.app/profile/" + sortedAPI.ParsedData.Author.Handle,
				URI:          "https://bsky.app/profile/" + sortedAPI.ParsedData.Author.Handle,
				Avatar:       sortedAPI.ParsedData.Author.Avatar,
				AvatarStatic: sortedAPI.ParsedData.Author.Avatar,
			},
			MediaAttachments: []types.RichActivityMedia{},
		}

		if sortedAPI.ParsedData.IsVideo {
			richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, types.RichActivityMedia{
				ID:          strconv.Itoa(rand.Int()),
				Type:        "video",
				URL:         sortedAPI.ParsedData.VideoHelper,
				Preview:     sortedAPI.ParsedData.Thumbnail,
				Description: "",
			})
		} else {
			if len(sortedAPI.ParsedData.Images) > 0 {
				for _, v := range sortedAPI.ParsedData.Images {
					richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, types.RichActivityMedia{
						ID:          strconv.Itoa(rand.Int()),
						Type:        "image",
						URL:         v.FullSize,
						Preview:     v.FullSize,
						Description: v.Alt,
					})
				}
			} else if sortedAPI.ParsedData.External.Thumb != "" {
				if sortedAPI.ParsedData.IsGif {
					richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, types.RichActivityMedia{
						ID:          strconv.Itoa(rand.Int()),
						Type:        "image",
						URL:         sortedAPI.ParsedData.External.URI,
						Preview:     sortedAPI.ParsedData.External.URI,
						Description: "",
					})
				} else {
					richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, types.RichActivityMedia{
						ID:          strconv.Itoa(rand.Int()),
						Type:        "image",
						URL:         sortedAPI.ParsedData.External.Thumb,
						Preview:     sortedAPI.ParsedData.External.Thumb,
						Description: "",
					})
				}
			} else if sortedAPI.ParsedData.CommonEmbeds.Avatar != "" {
				richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, types.RichActivityMedia{
					ID:          strconv.Itoa(rand.Int()),
					Type:        "image",
					URL:         sortedAPI.ParsedData.CommonEmbeds.Avatar,
					Preview:     sortedAPI.ParsedData.CommonEmbeds.Avatar,
					Description: "",
				})
			}
		}
	case "prof":
		newAPIReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fmt.Sprintf("https://api.%s/profile/%s", ps.DomainName, actReqData.Handle), http.NoBody)
		if err != nil {
			ErrorPage(w, "failed to request api data")
			return
		}

		apiResp, err := helpers.TimeoutClient.Do(newAPIReq)
		if err != nil {
			ErrorPage(w, "failed to do api request")
			return
		}

		defer apiResp.Body.Close()

		var sortedAPI types.UserProfile

		if decodeErr := json.NewDecoder(apiResp.Body).Decode(&sortedAPI); decodeErr != nil {
			ErrorPage(w, "failed to decode response")
			return
		}

		richContent := fmt.Sprintf("<p>%s</p><p>👥 %s Followers &ensp; 🌐 %s Following &ensp; ✍️ %s Posts</p>", sortedAPI.Description, helpers.ToNotation(sortedAPI.FollowersCount), helpers.ToNotation(sortedAPI.FollowsCount), helpers.ToNotation(sortedAPI.PostsCount))

		if sortedAPI.Associated.Labeler {
			richContent += "<p>🏷️ Labeler</p>"
		}

		richContent = strings.ReplaceAll(richContent, "\n", "<br>")

		richEmbed = types.RichActivity{
			ID:          strconv.Itoa(rand.Int()),
			URL:         "https://bsky.app/profile/" + sortedAPI.Handle,
			URI:         "https://bsky.app/profile/" + sortedAPI.Handle,
			CreatedAt:   sortedAPI.CreatedAt,
			Language:    "en",
			Content:     richContent,
			SpoilerText: "",
			Visibility:  "public",
			Application: types.RichActivityApplication{
				Name:    ps.DomainName,
				Website: "https://" + ps.DomainName,
			},
			Account: types.RichActivityAccount{
				ID:           strconv.Itoa(rand.Int()),
				DisplayName:  sortedAPI.DisplayName,
				UserName:     sortedAPI.Handle,
				Acct:         sortedAPI.Handle,
				URL:          "https://bsky.app/profile/" + sortedAPI.Handle,
				URI:          "https://bsky.app/profile/" + sortedAPI.Handle,
				Avatar:       sortedAPI.Avatar,
				AvatarStatic: sortedAPI.Avatar,
			},
			MediaAttachments: []types.RichActivityMedia{},
		}
	case "feed":
		newAPIReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fmt.Sprintf("https://api.%s/profile/%s/feed/%s", ps.DomainName, actReqData.Handle, actReqData.PostID), http.NoBody)
		if err != nil {
			ErrorPage(w, "failed to request api data")
			return
		}

		apiResp, err := helpers.TimeoutClient.Do(newAPIReq)
		if err != nil {
			ErrorPage(w, "failed to do api request")
			return
		}

		defer apiResp.Body.Close()

		var sortedAPI types.APIFeed

		if decodeErr := json.NewDecoder(apiResp.Body).Decode(&sortedAPI); decodeErr != nil {
			ErrorPage(w, "failed to decode response")
			return
		}

		richContent := fmt.Sprintf("<p>%s</p><p>🩷 %s Likes</p>", sortedAPI.View.Description, helpers.ToNotation(sortedAPI.View.LikeCount))

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

		richEmbed = types.RichActivity{
			ID:          strconv.Itoa(rand.Int()),
			URL:         fmt.Sprintf("https://bsky.app/profile/%s/feed/%s", sortedAPI.View.Creator.Handle, actReqData.PostID),
			URI:         fmt.Sprintf("https://bsky.app/profile/%s/feed/%s", sortedAPI.View.Creator.Handle, actReqData.PostID),
			CreatedAt:   sortedAPI.View.IndexedAt,
			Language:    "en",
			Content:     richContent,
			SpoilerText: "",
			Visibility:  "public",
			Application: types.RichActivityApplication{
				Name:    ps.DomainName,
				Website: "https://" + ps.DomainName,
			},
			Account: types.RichActivityAccount{
				ID:           strconv.Itoa(rand.Int()),
				DisplayName:  sortedAPI.View.DisplayName,
				UserName:     sortedAPI.View.Creator.Handle,
				Acct:         sortedAPI.View.Creator.Handle,
				URL:          "https://bsky.app/profile/" + sortedAPI.View.Creator.Handle,
				URI:          "https://bsky.app/profile/" + sortedAPI.View.Creator.Handle,
				Avatar:       sortedAPI.View.Creator.Avatar,
				AvatarStatic: sortedAPI.View.Creator.Avatar,
			},
			MediaAttachments: []types.RichActivityMedia{},
		}

		if sortedAPI.View.Avatar != "" {
			richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, types.RichActivityMedia{
				ID:          strconv.Itoa(rand.Int()),
				Type:        "image",
				URL:         sortedAPI.View.Avatar,
				Preview:     sortedAPI.View.Avatar,
				Description: "",
			})
		}
	case "list":
		newAPIReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fmt.Sprintf("https://api.%s/profile/%s/lists/%s", ps.DomainName, actReqData.Handle, actReqData.PostID), http.NoBody)
		if err != nil {
			ErrorPage(w, "failed to request api data")
			return
		}

		apiResp, err := helpers.TimeoutClient.Do(newAPIReq)
		if err != nil {
			ErrorPage(w, "failed to do api request")
			return
		}

		defer apiResp.Body.Close()

		var sortedAPI types.APIList

		if decodeErr := json.NewDecoder(apiResp.Body).Decode(&sortedAPI); decodeErr != nil {
			ErrorPage(w, "failed to decode response")
			return
		}

		richContent := fmt.Sprintf("<p>%s</p><p>👥 %s on List</p>", sortedAPI.List.Description, helpers.ToNotation(sortedAPI.List.ItemCount))

		richContent = strings.ReplaceAll(richContent, "\n", "<br>")

		richEmbed = types.RichActivity{
			ID:          strconv.Itoa(rand.Int()),
			URL:         fmt.Sprintf("https://bsky.app/profile/%s/feed/%s", sortedAPI.List.Creator.Handle, actReqData.PostID),
			URI:         fmt.Sprintf("https://bsky.app/profile/%s/feed/%s", sortedAPI.List.Creator.Handle, actReqData.PostID),
			CreatedAt:   sortedAPI.List.IndexedAt,
			Language:    "en",
			Content:     richContent,
			SpoilerText: "",
			Visibility:  "public",
			Application: types.RichActivityApplication{
				Name:    ps.DomainName,
				Website: "https://" + ps.DomainName,
			},
			Account: types.RichActivityAccount{
				ID:           strconv.Itoa(rand.Int()),
				DisplayName:  sortedAPI.List.Creator.DisplayName,
				UserName:     sortedAPI.List.Creator.Handle,
				Acct:         sortedAPI.List.Creator.Handle,
				URL:          "https://bsky.app/profile/" + sortedAPI.List.Creator.Handle,
				URI:          "https://bsky.app/profile/" + sortedAPI.List.Creator.Handle,
				Avatar:       sortedAPI.List.Creator.Avatar,
				AvatarStatic: sortedAPI.List.Creator.Avatar,
			},
			MediaAttachments: []types.RichActivityMedia{},
		}

		if sortedAPI.List.Avatar != "" {
			richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, types.RichActivityMedia{
				ID:          strconv.Itoa(rand.Int()),
				Type:        "image",
				URL:         sortedAPI.List.Avatar,
				Preview:     sortedAPI.List.Avatar,
				Description: "",
			})
		}
	case "pack":
		newAPIReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fmt.Sprintf("https://api.%s/starter-pack/%s/%s", ps.DomainName, actReqData.Handle, actReqData.PostID), http.NoBody)
		if err != nil {
			ErrorPage(w, "failed to request api data")
			return
		}

		apiResp, err := helpers.TimeoutClient.Do(newAPIReq)
		if err != nil {
			ErrorPage(w, "failed to do api request")
			return
		}

		defer apiResp.Body.Close()

		var sortedAPI types.APIPack

		if decodeErr := json.NewDecoder(apiResp.Body).Decode(&sortedAPI); decodeErr != nil {
			ErrorPage(w, "failed to decode response")
			return
		}

		richContent := fmt.Sprintf("<p>%s</p>", sortedAPI.StarterPack.Record.Description)

		richContent = strings.ReplaceAll(richContent, "\n", "<br>")

		richEmbed = types.RichActivity{
			ID:          strconv.Itoa(rand.Int()),
			URL:         fmt.Sprintf("https://bsky.app/starter-pack/%s/%s", sortedAPI.StarterPack.Creator.Handle, actReqData.PostID),
			URI:         fmt.Sprintf("https://bsky.app/starter-pack/%s/%s", sortedAPI.StarterPack.Creator.Handle, actReqData.PostID),
			CreatedAt:   sortedAPI.StarterPack.Record.CreatedAt,
			Language:    "en",
			Content:     richContent,
			SpoilerText: "",
			Visibility:  "public",
			Application: types.RichActivityApplication{
				Name:    ps.DomainName,
				Website: "https://" + ps.DomainName,
			},
			Account: types.RichActivityAccount{
				ID:           strconv.Itoa(rand.Int()),
				DisplayName:  sortedAPI.StarterPack.Creator.DisplayName,
				UserName:     sortedAPI.StarterPack.Creator.Handle,
				Acct:         sortedAPI.StarterPack.Creator.Handle,
				URL:          "https://bsky.app/profile/" + sortedAPI.StarterPack.Creator.Handle,
				URI:          "https://bsky.app/profile/" + sortedAPI.StarterPack.Creator.Handle,
				Avatar:       sortedAPI.StarterPack.Creator.Avatar,
				AvatarStatic: sortedAPI.StarterPack.Creator.Avatar,
			},
			MediaAttachments: []types.RichActivityMedia{},
		}

		ogCard := fmt.Sprintf("https://ogcard.cdn.bsky.app/start/%s/%s", sortedAPI.StarterPack.Creator.DID, actReqData.PostID)
		richEmbed.MediaAttachments = append(richEmbed.MediaAttachments, types.RichActivityMedia{
			ID:          strconv.Itoa(rand.Int()),
			Type:        "image",
			URL:         ogCard,
			Preview:     ogCard,
			Description: "",
		})
	default:
		ErrorPage(w, "Invalid type")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(&richEmbed)
}
