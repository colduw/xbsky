package handlers

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"main/internal/helpers"
	"main/internal/types"
)

var feedTemplate = template.Must(template.ParseFiles("./views/feed.html"))

func (ps *HandlerPass) GetFeed(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	feedID := r.PathValue("feedID")
	feedID = strings.ReplaceAll(feedID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = helpers.ResolveHandle(r.Context(), editedPID)
	}
	plcData := helpers.ResolvePLC(r.Context(), editedPID)

	if !strings.HasPrefix(editedPID, "at://") {
		editedPID = "at://" + editedPID
	}

	apiURL := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.feed.getFeedGenerator?feed=%s/app.bsky.feed.generator/%s", editedPID, feedID)
	if helpers.IsBlueskyDead.Load() {
		apiURL = fmt.Sprintf("https://api.bsky.app/xrpc/app.bsky.feed.getFeedGenerator?feed=%s/app.bsky.feed.generator/%s", editedPID, feedID)
	}

	req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		ErrorPage(w, "getFeed: failed to create request")
		return
	}

	resp, respErr := helpers.TimeoutClient.Do(req)
	if errors.Is(respErr, context.DeadlineExceeded) {
		ErrorPage(w, "getFeed: Bluesky took too long to respond (timeout exceeded)")
		return
	} else if respErr != nil {
		ErrorPage(w, "getFeed: failed to do request")
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ErrorPage(w, fmt.Sprintf("getFeed: Unexpected status (%s)", resp.Status))
		return
	}

	var feed types.APIFeed
	if decodeErr := json.NewDecoder(resp.Body).Decode(&feed); decodeErr != nil {
		ErrorPage(w, "getFeed: failed to decode response")
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

	encodedID := types.RichActivityEncoded{
		Type:   "feed",
		Handle: feed.View.Creator.DID,
		PostID: feedID,
	}

	marshaled, err := json.Marshal(encodedID)
	if err != nil {
		ErrorPage(w, "getFeed: failed to marshal for activity")
		return
	}

	feedTemplate.Execute(w, map[string]any{"feed": feed, "feedID": feedID, "isTelegram": isTelegramAgent, "encodedID": hex.EncodeToString(marshaled), "passData": ps})
}
