package handlers

import (
	"html/template"
	"net/http"
)

var errorTemplate = template.Must(template.ParseFiles("./views/error.html"))

func ErrorPage(w http.ResponseWriter, errorMessage string) {
	errorTemplate.Execute(w, map[string]string{"errorMsg": errorMessage})
}
