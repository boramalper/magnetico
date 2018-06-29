package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/boramalper/magnetico/pkg/persistence"
	"go.uber.org/zap"
)

func apiTorrentsHandler(w http.ResponseWriter, r *http.Request) {
	// @lastOrderedValue AND @lastID are either both supplied or neither of them should be supplied
	// at all; and if that is NOT the case, then return an error.
	if q := r.URL.Query(); !(
		(q.Get("lastOrderedValue") != "" && q.Get("lastID") != "") ||
			(q.Get("lastOrderedValue") == "" && q.Get("lastID") == "")) {
		respondError(w, 400, "`lastOrderedValue`, `lastID` must be supplied altogether, if supplied.")
		return
	}

	var tq TorrentsQ
	if err := decoder.Decode(&tq, r.URL.Query()); err != nil {
		respondError(w, 400, "error while parsing the URL: %s", err.Error())
		return
	}

	if tq.Query == nil {
		tq.Query = new(string)
		*tq.Query = ""
	}

	if tq.Epoch == nil {
		tq.Epoch = new(int64)
		*tq.Epoch = time.Now().Unix()  // epoch, if not supplied, is NOW.
	} else if *tq.Epoch <= 0 {
		respondError(w, 400, "epoch must be greater than 0")
		return
	}

	if tq.LastID != nil && *tq.LastID < 0 {
		respondError(w, 400, "lastID has to be greater than or equal to zero")
		return
	}

	if tq.Ascending == nil {
		tq.Ascending = new(bool)
		*tq.Ascending = true
	}

	torrents, err := database.QueryTorrents(
		*tq.Query, *tq.Epoch, persistence.ByRelevance,
		*tq.Ascending, N_TORRENTS, tq.LastOrderedValue, tq.LastID)
	if err != nil {
		respondError(w, 400, "query error: %s", err.Error())
		return
	}

	// TODO: use plain Marshal
	jm, err := json.MarshalIndent(torrents, "", "  ")
	if err != nil {
		respondError(w, 500, "json marshalling error: %s", err.Error())
		return
	}

	if _, err = w.Write(jm); err != nil {
		zap.L().Warn("couldn't write http.ResponseWriter", zap.Error(err))
	}
}

func apiTorrentsInfohashHandler(w http.ResponseWriter, r *http.Request) {

}

func apiFilesInfohashHandler(w http.ResponseWriter, r *http.Request) {

}

func apiStatisticsHandler(w http.ResponseWriter, r *http.Request) {

}
