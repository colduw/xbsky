package handlers

import (
	"net/http"
)

func (ps *HandlerPass) IndexPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		ErrorPage(w, "route not found")
		return
	}

	http.Redirect(w, r, ps.IndexURL, http.StatusFound)
}
