package main

import (
	"net/http"
	"net/url"
	"os"
	"time"

	"main/internal/handlers"
	"main/internal/helpers"

	"golang.org/x/crypto/acme/autocert"
)

func main() {
	if loadErr := helpers.LoadEnv(); loadErr != nil {
		panic(loadErr)
	}

	domainName := os.Getenv("DOMAIN_NAME")
	if domainName == "" {
		panic("DOMAIN_NAME environment variable should not be empty")
	}

	themeColor := os.Getenv("THEME_COLOR")
	if themeColor == "" {
		panic("THEME_COLOR environment variable should not be empty")
	}

	indexURL := os.Getenv("INDEX_URL")
	if indexURL == "" {
		panic("INDEX_URL environment variable should not be empty")
	}

	hPass := handlers.HandlerPass{
		DomainName: domainName,
		ThemeColor: themeColor,
		IndexURL:   indexURL,
	}

	sMux := http.NewServeMux()
	sMux.HandleFunc("GET /profile/{profileID}", hPass.GetProfile)
	sMux.HandleFunc("GET /profile/{profileID}/post/{postID}", hPass.GetPost)
	sMux.HandleFunc("GET /profile/{profileID}/post/{postID}/photo/{photoNum}", hPass.GetPost)
	sMux.HandleFunc("GET /profile/{profileID}/feed/{feedID}", hPass.GetFeed)
	sMux.HandleFunc("GET /profile/{profileID}/lists/{listID}", hPass.GetList)
	sMux.HandleFunc("GET /starter-pack/{profileID}/{packID}", hPass.GetPack)

	sMux.HandleFunc("GET /static/favicon.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./favicon.png")
	})

	sMux.HandleFunc("GET /users/{ignoredField}/statuses/{id}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://"+domainName+"/api/v1/statuses/"+url.PathEscape(r.PathValue("id")), http.StatusFound)
	})

	sMux.HandleFunc("GET /api/v1/statuses/{id}", hPass.GenActivity)
	sMux.HandleFunc("GET /oembed", hPass.GenOembed)
	sMux.HandleFunc("GET /", hPass.IndexPage)

	manager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domainName, "raw."+domainName, "mosaic."+domainName, "api."+domainName),
		Cache:      autocert.DirCache("certs"),
	}

	go helpers.BlueskyHealthCheck()

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
