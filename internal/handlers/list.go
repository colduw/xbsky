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

var listTemplate = template.Must(template.ParseFiles("./views/list.html"))

func (ps *HandlerPass) GetList(w http.ResponseWriter, r *http.Request) {
	profileID := r.PathValue("profileID")
	listID := r.PathValue("listID")
	listID = strings.ReplaceAll(listID, "|", "")

	editedPID := profileID
	if !strings.HasPrefix(editedPID, "did:plc") {
		editedPID = helpers.ResolveHandle(r.Context(), editedPID)
	}
	plcData := helpers.ResolvePLC(r.Context(), editedPID)

	if !strings.HasPrefix(editedPID, "at://") {
		editedPID = "at://" + editedPID
	}

	apiURL := fmt.Sprintf("https://public.api.bsky.app/xrpc/app.bsky.graph.getList?limit=1&list=%s/app.bsky.graph.list/%s", editedPID, listID)
	if helpers.IsBlueskyDead.Load() {
		apiURL = fmt.Sprintf("https://api.bsky.app/xrpc/app.bsky.graph.getList?limit=1&list=%s/app.bsky.graph.list/%s", editedPID, listID)
	}

	req, reqErr := http.NewRequestWithContext(r.Context(), http.MethodGet, apiURL, http.NoBody)
	if reqErr != nil {
		ErrorPage(w, "getList: failed to create request")
		return
	}

	resp, respErr := helpers.TimeoutClient.Do(req)
	if errors.Is(respErr, context.DeadlineExceeded) {
		ErrorPage(w, "getList: Bluesky took too long to respond (timeout exceeded)")
		return
	} else if respErr != nil {
		ErrorPage(w, "getList: failed to do request")
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ErrorPage(w, fmt.Sprintf("getList: Unexpected status (%s)", resp.Status))
		return
	}

	var list types.APIList
	if decodeErr := json.NewDecoder(resp.Body).Decode(&list); decodeErr != nil {
		ErrorPage(w, "getList: failed to decode response")
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

	encodedID := types.RichActivityEncoded{
		Type:   "list",
		Handle: list.List.Creator.DID,
		PostID: listID,
	}

	marshaled, err := json.Marshal(encodedID)
	if err != nil {
		ErrorPage(w, "getList: failed to marshal for activity")
		return
	}

	listTemplate.Execute(w, map[string]any{"list": list.List, "listID": listID, "isTelegram": isTelegramAgent, "encodedID": hex.EncodeToString(marshaled), "passData": ps})
}
