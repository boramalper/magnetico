package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/boramalper/magnetico/pkg/persistence"
)

// DONE
func rootHandler(w http.ResponseWriter, r *http.Request) {
	nTorrents, err := database.GetNumberOfTorrents()
	if err != nil {
		handlerError(errors.Wrap(err, "GetNumberOfTorrents"), w)
		return
	}

	_ = templates["homepage"].Execute(w, struct {
		NTorrents uint
	}{
		NTorrents: nTorrents,
	})
}

func torrentsHandler(w http.ResponseWriter, r *http.Request) {
	data := mustAsset("templates/torrents.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Cache static resources for a day
	w.Header().Set("Cache-Control", "max-age=86400")
	_, _ = w.Write(data)
}

func torrentsInfohashHandler(w http.ResponseWriter, r *http.Request) {
	data := mustAsset("templates/torrent.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Cache static resources for a day
	w.Header().Set("Cache-Control", "max-age=86400")
	_, _ = w.Write(data)
	return
}

func statisticsHandler(w http.ResponseWriter, r *http.Request) {
	data := mustAsset("templates/statistics.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Cache static resources for a day
	w.Header().Set("Cache-Control", "max-age=86400")
	_, _ = w.Write(data)
}

func feedHandler(w http.ResponseWriter, r *http.Request) {
	var query, title string
	switch len(r.URL.Query()["query"]) {
	case 0:
		query = ""
	case 1:
		query = r.URL.Query()["query"][0]
	default:
		respondError(w, 400, "query supplied multiple times!")
		return
	}

	if query == "" {
		title = "Most recent torrents - magneticow"
	} else {
		title = "`" + query + "` - magneticow"
	}

	torrents, err := database.QueryTorrents(
		query,
		time.Now().Unix(),
		persistence.ByDiscoveredOn,
		false,
		20,
		nil,
		nil,
	)
	if err != nil {
		handlerError(errors.Wrap(err, "query torrent"), w)
		return
	}

	// It is much more convenient to write the XML deceleration manually*, and then process the XML
	// template using template/html and send, than to use encoding/xml.
	//
	// *: https://github.com/golang/go/issues/3133
	//
	// TODO: maybe do it properly, even if it's inconvenient?
	_, _ = w.Write([]byte(`<?xml version="1.0" encoding="utf-8" standalone="yes"?>`))
	_ = templates["feed"].Execute(w, struct {
		Title    string
		Torrents []persistence.TorrentMetadata
	}{
		Title:    title,
		Torrents: torrents,
	})
}

func staticHandler(w http.ResponseWriter, r *http.Request) {
	data, err := Asset(r.URL.Path[1:])
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var contentType string
	if strings.HasSuffix(r.URL.Path, ".css") {
		contentType = "text/css; charset=utf-8"
	} else if strings.HasSuffix(r.URL.Path, ".js") {
		contentType = "text/javascript; charset=utf-8"
	} else { // fallback option
		contentType = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", contentType)
	// Cache static resources for a day
	w.Header().Set("Cache-Control", "max-age=86400")
	_, _ = w.Write(data)
}
