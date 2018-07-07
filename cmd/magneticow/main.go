package main

import (
	"encoding/hex"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/boramalper/magnetico/pkg/persistence"
)

const N_TORRENTS = 20

// Set a Decoder instance as a package global, because it caches
// meta-data about structs, and an instance can be shared safely.
var decoder = schema.NewDecoder()

var templates map[string]*template.Template
var database persistence.Database

// ======= Q: Query =======
type TorrentsQ struct {
	Epoch            *int64   `schema:"epoch"`
	Query            *string  `schema:"query"`
	OrderBy          *string  `schema:"orderBy"`
	Ascending        *bool    `schema:"ascending"`
	LastOrderedValue *float64 `schema:"lastOrderedValue"`
	LastID           *uint64  `schema:"lastID"`
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

	router.HandleFunc("/api/v0.1/torrents", apiTorrentsHandler)
	router.HandleFunc("/api/v0.1/torrents/{infohash:[a-z0-9]{40}}", apiTorrentsInfohashHandler)
	router.HandleFunc("/api/v0.1/files/{infohash:[a-z0-9]{40}}", apiFilesInfohashHandler)
	router.HandleFunc("/api/v0.1/statistics", apiStatisticsHandler)

	router.HandleFunc("/torrents", torrentsHandler)
	router.HandleFunc("/torrents/{infohash:[a-z0-9]{40}}", torrentsInfohashHandler)
	router.HandleFunc("/statistics", statisticsHandler)
	router.HandleFunc("/feed", feedHandler)

	router.PathPrefix("/static").HandlerFunc(staticHandler)

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

		"humanizeSizeF": func(s int64) string {
			if s < 0 {
				return ""
			}
			return humanize.IBytes(uint64(s))
		},

		"comma": func(s uint) string {
			return humanize.Comma(int64(s))
		},
	}

	templates = make(map[string]*template.Template)
	templates["feed"] = template.Must(template.New("feed").Funcs(templateFunctions).Parse(string(mustAsset("templates/feed.xml"))))
	templates["homepage"] = template.Must(template.New("homepage").Funcs(templateFunctions).Parse(string(mustAsset("templates/homepage.html"))))
	// templates["statistics"] = template.Must(template.New("statistics").Parse(string(mustAsset("templates/statistics.html"))))
	templates["torrent"] = template.Must(template.New("torrent").Funcs(templateFunctions).Parse(string(mustAsset("templates/torrent.html"))))
	// templates["torrents"] = template.Must(template.New("torrents").Funcs(templateFunctions).Parse(string(mustAsset("templates/torrents.html"))))

	var err error
	database, err = persistence.MakeDatabase("sqlite3:///home/bora/.local/share/magneticod/database.sqlite3", logger)
	if err != nil {
		panic(err.Error())
	}

	decoder.IgnoreUnknownKeys(false)
	decoder.ZeroEmpty(true)

	zap.L().Info("magneticow is ready to serve!")
	err = http.ListenAndServe(":10101", router)
	if err != nil {
		zap.L().Error("ListenAndServe error", zap.Error(err))
	}
}

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

// TODO: I think there is a standard lib. function for this
func respondError(w http.ResponseWriter, statusCode int, format string, a ...interface{}) {
	w.WriteHeader(statusCode)
	w.Write([]byte(fmt.Sprintf(format, a...)))
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
		Title: title,
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

func mustAsset(name string) []byte {
	data, err := Asset(name)
	if err != nil {
		log.Panicf("Could NOT access the requested resource `%s`: %s (please inform us, this is a BUG!)", name, err.Error())
	}
	return data
}
