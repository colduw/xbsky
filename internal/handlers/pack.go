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

var packTemplate = template.Must(template.ParseFiles("./views/pack.html"))

func (ps *HandlerPass) GetPack(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	packID := r.PathValue("packID")
	packID = strings.ReplaceAll(packID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = helpers.ResolveHandle(r.Context(), editedPID)
	}
	plcData := helpers.ResolvePLC(r.Context(), editedPID)

	if !strings.HasPrefix(editedPID, "at://") {
		editedPID = "at://" + editedPID
	}

	apiURL := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.graph.getStarterPack?starterPack=%s/app.bsky.graph.starterpack/%s", editedPID, packID)
	if helpers.IsBlueskyDead.Load() {
		apiURL = fmt.Sprintf("https://api.bsky.app/xrpc/app.bsky.graph.getStarterPack?starterPack=%s/app.bsky.graph.starterpack/%s", editedPID, packID)
	}

	req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		ErrorPage(w, "getPack: failed to create request")
		return
	}

	resp, respErr := helpers.TimeoutClient.Do(req)
	if errors.Is(respErr, context.DeadlineExceeded) {
		ErrorPage(w, "getPack: Bluesky took too long to respond (timeout exceeded)")
		return
	} else if respErr != nil {
		ErrorPage(w, "getPack: failed to do request")
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ErrorPage(w, fmt.Sprintf("getPack: Unexpected status (%s)", resp.Status))
		return
	}

	var pack types.APIPack
	if decodeErr := json.NewDecoder(resp.Body).Decode(&pack); decodeErr != nil {
		ErrorPage(w, "getPack: failed to decode response")
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

	encodedID := types.RichActivityEncoded{
		Type:   "pack",
		Handle: pack.StarterPack.Creator.DID,
		PostID: packID,
	}

	marshaled, err := json.Marshal(encodedID)
	if err != nil {
		ErrorPage(w, "getPack: failed to marshal for activity")
		return
	}

	packTemplate.Execute(w, map[string]any{"pack": pack.StarterPack, "packID": packID, "isTelegram": isTelegramAgent, "encodedID": hex.EncodeToString(marshaled), "passData": ps})
}
