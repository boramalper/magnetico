package main

import (
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	//"strconv"
	"strings"
	// "time"

	"github.com/dustin/go-humanize"
	// "github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/boramalper/magnetico/pkg/persistence"
)

const N_TORRENTS = 20

var templates map[string]*template.Template
var database persistence.Database

// ========= TD: TemplateData =========
type HomepageTD struct {
	NTorrents uint
}

type TorrentsTD struct {
	CanLoadMore      bool
	Query            string
	SubscriptionURL  string
	Torrents         []persistence.TorrentMetadata
	SortedBy         string
	NextPageExists   bool
	Epoch            int64
	LastOrderedValue uint64
	LastID           uint64

}

type TorrentTD struct {
}

type FeedTD struct {
}

type StatisticsTD struct {
}

func main() {
	loggerLevel := zap.NewAtomicLevel()
	// Logging levels: ("debug", "info", "warn", "error", "dpanic", "panic", and "fatal").
	logger := zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.Lock(os.Stderr),
		loggerLevel,
	))
	defer logger.Sync()
	zap.ReplaceGlobals(logger)

	zap.L().Info("magneticow v0.7.0 has been started.")
	zap.L().Info("Copyright (C) 2017  Mert Bora ALPER <bora@boramalper.org>.")
	zap.L().Info("Dedicated to Cemile Binay, in whose hands I thrived.")

	router := mux.NewRouter()
	router.HandleFunc("/", rootHandler)
	router.HandleFunc("/torrents", torrentsHandler)
	router.HandleFunc("/torrents/{infohash:[a-z0-9]{40}}", torrentsInfohashHandler)
	router.HandleFunc("/statistics", statisticsHandler)
	router.PathPrefix("/static").HandlerFunc(staticHandler)

	router.HandleFunc("/feed", feedHandler)

	templateFunctions := template.FuncMap{
		"add": func(augend int, addends int) int {
			return augend + addends
		},

		"subtract": func(minuend int, subtrahend int) int {
			return minuend - subtrahend
		},

		"bytesToHex": func(bytes []byte) string {
			return hex.EncodeToString(bytes)
		},

		"unixTimeToYearMonthDay": func(s int64) string {
			tm := time.Unix(s, 0)
			// > Format and Parse use example-based layouts. Usually you’ll use a constant from time
			// > for these layouts, but you can also supply custom layouts. Layouts must use the
			// > reference time Mon Jan 2 15:04:05 MST 2006 to show the pattern with which to
			// > format/parse a given time/string. The example time must be exactly as shown: the
			// > year 2006, 15 for the hour, Monday for the day of the week, etc.
			// https://gobyexample.com/time-formatting-parsing
			// Why you gotta be so weird Go?
			return tm.Format("02/01/2006")
		},

		"humanizeSize": func(s uint64) string {
			return humanize.IBytes(s)
		},

		"comma": func(s uint) string {
			return humanize.Comma(int64(s))
		},
	}

	templates = make(map[string]*template.Template)
	// templates["feed"] = template.Must(template.New("feed").Parse(string(mustAsset("templates/feed.xml"))))
	templates["homepage"] = template.Must(template.New("homepage").Funcs(templateFunctions).Parse(string(mustAsset("templates/homepage.html"))))
	// templates["statistics"] = template.Must(template.New("statistics").Parse(string(mustAsset("templates/statistics.html"))))
	// templates["torrent"] = template.Must(template.New("torrent").Funcs(templateFunctions).Parse(string(mustAsset("templates/torrent.html"))))
	templates["torrents"] = template.Must(template.New("torrents").Funcs(templateFunctions).Parse(string(mustAsset("templates/torrents.html"))))

	var err error
	database, err = persistence.MakeDatabase("sqlite3:///home/bora/.local/share/magneticod/database.sqlite3", logger)
	if err != nil {
		panic(err.Error())
	}

	zap.L().Info("magneticow is ready to serve!")
	http.ListenAndServe(":8080", router)
}

// DONE
func rootHandler(w http.ResponseWriter, r *http.Request) {
	nTorrents, err := database.GetNumberOfTorrents()
	if err != nil {
		panic(err.Error())
	}
	templates["homepage"].Execute(w, HomepageTD{
		NTorrents: nTorrents,
	})
}

func respondError(w http.ResponseWriter, statusCode int, format string, a ...interface{}) {
	w.WriteHeader(statusCode)
	w.Write([]byte(fmt.Sprintf(format, a...)))
}

func torrentsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Parsing URL Query is tedious and looks stupid... can we do better?
	queryValues := r.URL.Query()

	var query string
	epoch := time.Now().Unix()  // epoch, if not supplied, is NOW.
	var lastOrderedValue, lastID *uint64

	if query = queryValues.Get("query"); query == "" {
		respondError(w, 400, "query is missing")
		return
	}

	if queryValues.Get("epoch") != "" && queryValues.Get("lastOrderedValue") != "" && queryValues.Get("lastID") != "" {
		var err error

		epoch, err = strconv.ParseInt(queryValues.Get("epoch"), 10, 64)
		if err != nil {
			respondError(w, 400, "error while parsing epoch: %s", err.Error())
			return
		}
		if epoch <= 0 {
			respondError(w, 400, "epoch has to be greater than zero")
			return
		}

		*lastOrderedValue, err = strconv.ParseUint(queryValues.Get("lastOrderedValue"), 10, 64)
		if err != nil {
			respondError(w, 400, "error while parsing lastOrderedValue: %s", err.Error())
			return
		}
		if *lastOrderedValue <= 0 {
			respondError(w, 400, "lastOrderedValue has to be greater than zero")
			return
		}

		*lastID, err = strconv.ParseUint(queryValues.Get("lastID"), 10, 64)
		if err != nil {
			respondError(w, 400, "error while parsing lastID: %s", err.Error())
			return
		}
		if *lastID <= 0 {
			respondError(w, 400, "lastID has to be greater than zero")
			return
		}
	} else if !(queryValues.Get("epoch") == "" && queryValues.Get("lastOrderedValue") == "" && queryValues.Get("lastID") == "") {
		respondError(w, 400, "`epoch`, `lastOrderedValue`, `lastID` must be supplied altogether, if supplied.")
		return
	}

	torrents, err := database.QueryTorrents(query, epoch, persistence.ByRelevance, true, 20, nil, nil)
	if err != nil {
		respondError(w, 400, "query error: %s", err.Error())
		return
	}

	if torrents == nil {
		panic("torrents is nil!!!")
	}

	templates["torrents"].Execute(w, TorrentsTD{
		CanLoadMore:     true,
		Query:           query,
		SubscriptionURL: "borabora",
		Torrents:        torrents,
		SortedBy:        "anan",
		NextPageExists:  true,
	})
}

func torrentsInfohashHandler(w http.ResponseWriter, r *http.Request) {
	// show torrents/{infohash}
	infoHash, err := hex.DecodeString(mux.Vars(r)["infohash"])
	if err != nil {
		panic(err.Error())
	}

	torrent, err := database.GetTorrent(infoHash)
	if err != nil {
		panic(err.Error())
	}

	templates["torrent"].Execute(w, torrent)
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
	} else { // fallback option
		contentType = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", contentType)
	w.Write(data)
}

func mustAsset(name string) []byte {
	data, err := Asset(name)
	if err != nil {
		log.Panicf("Could NOT access the requested resource `%s`: %s (please inform us, this is a BUG!)", name, err.Error())
	}
	return data
}
