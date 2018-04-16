package main

import (
	"encoding/hex"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/dustin/go-humanize"
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
	Count uint
}

type TorrentsTD struct {
	Search          string
	SubscriptionURL string
	Torrents        []persistence.TorrentMetadata
	Before          int64
	After           int64
	SortedBy        string
	NextPageExists  bool
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
	}

	templates = make(map[string]*template.Template)
	templates["feed"] = template.Must(template.New("feed").Parse(string(mustAsset("templates/feed.xml"))))
	templates["homepage"] = template.Must(template.New("homepage").Parse(string(mustAsset("templates/homepage.html"))))
	templates["statistics"] = template.Must(template.New("statistics").Parse(string(mustAsset("templates/statistics.html"))))
	templates["torrent"] = template.Must(template.New("torrent").Funcs(templateFunctions).Parse(string(mustAsset("templates/torrent.html"))))
	templates["torrents"] = template.Must(template.New("torrents").Funcs(templateFunctions).Parse(string(mustAsset("templates/torrents.html"))))

	var err error
	database, err = persistence.MakeDatabase("sqlite3:///home/bora/.local/share/magneticod/database.sqlite3", unsafe.Pointer(logger))
	if err != nil {
		panic(err.Error())
	}

	zap.L().Info("magneticow is ready to serve!")
	http.ListenAndServe(":8080", router)
}

// DONE
func rootHandler(w http.ResponseWriter, r *http.Request) {
	count, err := database.GetNumberOfTorrents()
	if err != nil {
		panic(err.Error())
	}
	templates["homepage"].Execute(w, HomepageTD{
		Count: count,
	})
}

func torrentsHandler(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()

	// Parses `before` and `after` parameters in the URL query following the conditions below:
	// * `before` and `after` cannot be both supplied at the same time.
	// * `before` -if supplied- cannot be less than or equal to zero.
	// * `after` -if supplied- cannot be greater than the current Unix time.
	// * if `before` is not supplied, it is set to the current Unix time.
	qBefore, qAfter := (int64)(-1), (int64)(-1)
	var err error
	if queryValues.Get("before") != "" {
		qBefore, err = strconv.ParseInt(queryValues.Get("before"), 10, 64)
		if err != nil {
			panic(err.Error())
		}
		if qBefore <= 0 {
			panic("before parameter is less than or equal to zero!")
		}
	} else if queryValues.Get("after") != "" {
		if qBefore != -1 {
			panic("both before and after supplied")
		}
		qAfter, err = strconv.ParseInt(queryValues.Get("after"), 10, 64)
		if err != nil {
			panic(err.Error())
		}
		if qAfter > time.Now().Unix() {
			panic("after parameter is greater than the current Unix time!")
		}
	} else {
		qBefore = time.Now().Unix()
	}

	var torrents []persistence.TorrentMetadata
	if qBefore != -1 {
		torrents, err = database.GetNewestTorrents(N_TORRENTS, qBefore)
	} else {
		torrents, err = database.QueryTorrents(
			queryValues.Get("search"),
			persistence.BY_DISCOVERED_ON,
			true,
			false,
			N_TORRENTS,
			qAfter,
			true,
		)
	}
	if err != nil {
		panic(err.Error())
	}

	// TODO: for testing, REMOVE
	torrents[2].HasReadme = true

	templates["torrents"].Execute(w, TorrentsTD{
		Search:          "",
		SubscriptionURL: "borabora",
		Torrents:        torrents,
		Before:          torrents[len(torrents)-1].DiscoveredOn,
		After:           torrents[0].DiscoveredOn,
		SortedBy:        "anan",
		NextPageExists:  true,
	})

}

func newestTorrentsHandler(w http.ResponseWriter, r *http.Request) {
	queryValues := r.URL.Query()

	qBefore, qAfter := (int64)(-1), (int64)(-1)
	var err error
	if queryValues.Get("before") != "" {
		qBefore, err = strconv.ParseInt(queryValues.Get("before"), 10, 64)
		if err != nil {
			panic(err.Error())
		}
	} else if queryValues.Get("after") != "" {
		if qBefore != -1 {
			panic("both before and after supplied")
		}
		qAfter, err = strconv.ParseInt(queryValues.Get("after"), 10, 64)
		if err != nil {
			panic(err.Error())
		}
	} else {
		qBefore = time.Now().Unix()
	}

	var torrents []persistence.TorrentMetadata
	if qBefore != -1 {
		torrents, err = database.QueryTorrents(
			"",
			persistence.BY_DISCOVERED_ON,
			true,
			false,
			N_TORRENTS,
			qBefore,
			false,
		)
	} else {
		torrents, err = database.QueryTorrents(
			"",
			persistence.BY_DISCOVERED_ON,
			false,
			false,
			N_TORRENTS,
			qAfter,
			true,
		)
	}
	if err != nil {
		panic(err.Error())
	}

	templates["torrents"].Execute(w, TorrentsTD{
		Search:          "",
		SubscriptionURL: "borabora",
		Torrents:        torrents,
		Before:          torrents[len(torrents)-1].DiscoveredOn,
		After:           torrents[0].DiscoveredOn,
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
