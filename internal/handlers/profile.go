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

var profileTemplate = template.Must(template.ParseFiles("./views/profile.html"))

func (ps *HandlerPass) GetProfile(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	profileID = strings.ReplaceAll(profileID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = helpers.ResolveHandle(r.Context(), editedPID)
	}
	plcData := helpers.ResolvePLC(r.Context(), editedPID)

	apiURL := "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=" + editedPID
	if helpers.IsBlueskyDead.Load() {
		apiURL = "https://api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=" + editedPID
	}

	req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		ErrorPage(w, "getProfile: Failed to create request")
		return
	}

	resp, respErr := helpers.TimeoutClient.Do(req)
	if errors.Is(respErr, context.DeadlineExceeded) {
		ErrorPage(w, "getProfile: Bluesky took too long to respond (timeout exceeded)")
		return
	} else if respErr != nil {
		ErrorPage(w, "getProfile: Failed to do request")
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ErrorPage(w, fmt.Sprintf("getProfile: Unexpected status (%s)", resp.Status))
		return
	}

	var profile types.UserProfile
	if decodeErr := json.NewDecoder(resp.Body).Decode(&profile); decodeErr != nil {
		ErrorPage(w, "getProfile: Failed to decode response")
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

	encodedID := types.RichActivityEncoded{
		Type:   "prof",
		Handle: profile.Handle,
	}

	marshaled, err := json.Marshal(encodedID)
	if err != nil {
		ErrorPage(w, "getProfile: failed to marshal for activity")
		return
	}

	profileTemplate.Execute(w, map[string]any{"profile": profile, "isTelegram": isTelegramAgent, "encodedID": hex.EncodeToString(marshaled), "passData": ps})
}
