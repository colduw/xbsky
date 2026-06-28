package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"main/internal/helpers"
	"main/internal/types"
)

func (ps *HandlerPass) GenOembed(w http.ResponseWriter, r *http.Request) {
	media := r.URL.Query().Get("for")

	embed := types.OEmbed{
		Version:      "1.0",
		Type:         "link",
		ProviderName: ps.DomainName,
		ProviderURL:  "https://" + ps.DomainName,
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

		embed.AuthorName = fmt.Sprintf("👥 %s Followers - 🌐 %s Following - ✍️ %s Posts", helpers.ToNotation(followers), helpers.ToNotation(follows), helpers.ToNotation(posts))

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

		embed.AuthorName = fmt.Sprintf("💬 %s   🔁 %s   🩷 %s   📝 %s", helpers.ToNotation(replies), helpers.ToNotation(reposts), helpers.ToNotation(likes), helpers.ToNotation(quotes))

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

		embed.AuthorName = fmt.Sprintf("🩷 %s Likes", helpers.ToNotation(likes))

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
