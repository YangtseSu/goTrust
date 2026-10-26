// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"bytes"
	"net/http"
)

// page is one question: a mux reachable only under its token and the channel
// its answer arrives on.
type page[T any] struct {
	token   string
	mux     *http.ServeMux
	answers chan T
}

func newPage[T any](token string) *page[T] {
	return &page[T]{
		token:   token,
		mux:     http.NewServeMux(),
		answers: make(chan T, 1),
	}
}

func (p *page[T]) path() string {
	return "/" + p.token + "/"
}

// addForm registers the page and its answer endpoint. render builds the HTML
// with a failure to show, if any; parse turns a submitted form into an answer
// and returns the reason to show when the form is not usable. A page answers
// once: a second submission is a conflict.
func addForm[T any](p *page[T], render func(failure string) []byte, parse func(r *http.Request) (T, error)) {
	p.mux.HandleFunc("GET "+p.path()+"{$}", func(w http.ResponseWriter, r *http.Request) {
		writePage(w, http.StatusOK, render(""))
	})
	p.mux.HandleFunc("POST "+p.path()+"{$}", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
		if err := r.ParseForm(); err != nil {
			writePage(w, http.StatusBadRequest, render("表单无法解析，请重试。"))
			return
		}
		answer, err := parse(r)
		if err != nil {
			writePage(w, http.StatusBadRequest, render(err.Error()))
			return
		}
		select {
		case p.answers <- answer:
			writePage(w, http.StatusOK, []byte(successHTML))
		default:
			http.Error(w, "this prompt has already been answered", http.StatusConflict)
		}
	})
}

// addImage serves the challenge's image, unchanged and at its natural size:
// the click page maps the pointer back to those pixels.
func addImage[T any](p *page[T], image []byte) {
	p.mux.HandleFunc("GET "+p.path()+"image", func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Type", http.DetectContentType(image))
		header.Set("Cache-Control", "no-store")
		header.Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(image)
	})
}

// writePage sends one page with the headers that keep a local prompt out of
// caches, referrers and other origins' reach. The page loads nothing from the
// network: its style, script and image are same-origin or inline.
func writePage(w http.ResponseWriter, status int, body []byte) {
	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("Pragma", "no-cache")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Content-Security-Policy",
		"default-src 'none'; img-src 'self'; style-src 'unsafe-inline'; "+
			"script-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	w.WriteHeader(status)
	if body == nil {
		body = []byte("this page could not be rendered")
	}
	_, _ = w.Write(body)
}

// renderPage executes one named page template.
func renderPage(name string, data any) []byte {
	var buf bytes.Buffer
	if err := pageTemplates.ExecuteTemplate(&buf, name, data); err != nil {
		return nil
	}
	return buf.Bytes()
}
