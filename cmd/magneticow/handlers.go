package main

import (
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/boramalper/magnetico/pkg/persistence"
	"github.com/gorilla/mux"
)

// DONE
func rootHandler(w http.ResponseWriter, r *http.Request) {
	nTorrents, err := database.GetNumberOfTorrents()
	if err != nil {
		panic(err.Error())
	}

	err = templates["homepage"].Execute(w, struct {
		NTorrents uint
	}{
		NTorrents: nTorrents,
	})
	if err != nil {
		panic(err.Error())
	}
}

// TODO: we might as well move torrents.html into static...
func torrentsHandler(w http.ResponseWriter, r *http.Request) {
	data := mustAsset("templates/torrents.html")
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Write(data)
}

func torrentsInfohashHandler(w http.ResponseWriter, r *http.Request) {
	infoHash, err := hex.DecodeString(mux.Vars(r)["infohash"])
	if err != nil {
		panic(err.Error())
	}

	torrent, err := database.GetTorrent(infoHash)
	if err != nil {
		panic(err.Error())
	}
	if torrent == nil {
		w.WriteHeader(404)
		w.Write([]byte("torrent not found!"))
		return
	}

	files, err := database.GetFiles(infoHash)
	if err != nil {
		panic(err.Error())
	}
	if files == nil {
		w.WriteHeader(500)
		w.Write([]byte("files not found what!!!"))
		return
	}

	err = templates["torrent"].Execute(w, struct {
		T *persistence.TorrentMetadata
		F []persistence.File
	}{
		T: torrent,
		F: files,
	})
	if err != nil {
		panic("error while executing template!")
	}
}

// TODO: we might as well move statistics.html into static...
func statisticsHandler(w http.ResponseWriter, r *http.Request) {
	data := mustAsset("templates/statistics.html")
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Write(data)
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
		N_TORRENTS,
		nil,
		nil,
	)
	if err != nil {
		respondError(w, 400, err.Error())
		return
	}

	// It is much more convenient to write the XML deceleration manually*, and then process the XML
	// template using template/html and send, then to use encoding/xml.
	//
	// *: https://github.com/golang/go/issues/3133
	//
	// TODO: maybe do it properly, even if it's inconvenient?

	_, err = w.Write([]byte(`<?xml version="1.0" encoding="utf-8" standalone="yes"?>`))
	if err != nil {
		panic(err.Error())
	}

	err = templates["feed"].Execute(w, struct {
		Title    string
		Torrents []persistence.TorrentMetadata
	}{
		Title:    title,
		Torrents: torrents,
	})
	if err != nil {
		panic(err.Error())
	}
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
	} else { // fallback option
		contentType = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}
