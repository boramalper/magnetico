package main

import (
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"persistence"
)

const N_TORRENTS = 20

var templates map[string]*template.Template
var database persistence.Database

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/", rootHandler)
	router.HandleFunc("/torrents", torrentsHandler)
	router.HandleFunc("/torrents/{infohash}", torrentsInfohashHandler)
	router.HandleFunc("/torrents/{infohash}/{name}", torrentsInfohashNameHandler)
	router.HandleFunc("/statistics", statisticsHandler)
	router.PathPrefix("/static").HandlerFunc(staticHandler)

	router.HandleFunc("/feed", feedHandler)

	templates = make(map[string]*template.Template)
	templates["feed"]       = template.Must(template.New("feed").Parse(string(mustAsset("templates/feed.xml"))))
	templates["homepage"]   = template.Must(template.New("homepage").Parse(string(mustAsset("templates/homepage.html"))))
	templates["statistics"] = template.Must(template.New("statistics").Parse(string(mustAsset("templates/statistics.html"))))
	templates["torrent"]    = template.Must(template.New("torrent").Parse(string(mustAsset("templates/torrent.html"))))
	templates["torrents"]   = template.Must(template.New("torrents").Parse(string(mustAsset("templates/torrents.html"))))

	var err error
	database, err = persistence.MakeDatabase("sqlite3:///home/bora/.local/share/magneticod/database.sqlite3")
	if err != nil {
		panic(err.Error())
	}

	http.ListenAndServe(":8080", router)
}


func rootHandler(w http.ResponseWriter, r *http.Request) {
	count, err := database.GetNumberOfTorrents()
	if err != nil {
		panic(err.Error())
	}
	templates["homepage"].Execute(w, count)
}


func torrentsHandler(w http.ResponseWriter, r *http.Request) {
	/*
	newestTorrents, err := database.NewestTorrents(N_TORRENTS)
	if err != nil {
		panic(err.Error())
	}
	templates["torrents"].Execute(w, nil)
	*/
}

func torrentsInfohashHandler(w http.ResponseWriter, r *http.Request) {
	// redirect to torrents/{infohash}/name
}


func torrentsInfohashNameHandler(w http.ResponseWriter, r *http.Request) {

}


func statisticsHandler(w http.ResponseWriter, r *http.Request) {

}


func feedHandler(w http.ResponseWriter, r *http.Request) {

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
	} else {  // fallback option
		contentType = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}

func mustAsset(name string) []byte {
	data, err := Asset(name)
	if err != nil {
		log.Panicf("Could NOT access the requested resource `%s`: %s", name, err.Error())
	}
	return data
}
