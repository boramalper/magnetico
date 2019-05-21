package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/boramalper/magnetico/pkg/persistence"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

func apiTorrents(w http.ResponseWriter, r *http.Request) {
	// @lastOrderedValue AND @lastID are either both supplied or neither of them should be supplied
	// at all; and if that is NOT the case, then return an error.
	if q := r.URL.Query(); !((q.Get("lastOrderedValue") != "" && q.Get("lastID") != "") ||
		(q.Get("lastOrderedValue") == "" && q.Get("lastID") == "")) {
		respondError(w, 400, "`lastOrderedValue`, `lastID` must be supplied altogether, if supplied.")
		return
	}

	var tq struct {
		Epoch            *int64   `schema:"epoch"`
		Query            *string  `schema:"query"`
		OrderBy          *string  `schema:"orderBy"`
		Ascending        *bool    `schema:"ascending"`
		LastOrderedValue *float64 `schema:"lastOrderedValue"`
		LastID           *uint64  `schema:"lastID"`
		Limit            *uint    `schema:"limit"`
	}
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
		*tq.Epoch = time.Now().Unix() // epoch, if not supplied, is NOW.
	} else if *tq.Epoch <= 0 {
		respondError(w, 400, "epoch must be greater than 0")
		return
	}

	if tq.Ascending == nil {
		tq.Ascending = new(bool)
		*tq.Ascending = true
	}

	var orderBy persistence.OrderingCriteria
	if tq.OrderBy == nil {
		if *tq.Query == "" {
			orderBy = persistence.ByDiscoveredOn
		} else {
			orderBy = persistence.ByRelevance
		}
	} else {
		var err error
		orderBy, err = parseOrderBy(*tq.OrderBy)
		if err != nil {
			respondError(w, 400, err.Error())
			return
		}
	}

	if tq.Limit == nil {
		tq.Limit = new(uint)
		*tq.Limit = 20
	}

	torrents, err := database.QueryTorrents(
		*tq.Query, *tq.Epoch, orderBy,
		*tq.Ascending, *tq.Limit, tq.LastOrderedValue, tq.LastID)
	if err != nil {
		respondError(w, 400, "query error: %s", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err = json.NewEncoder(w).Encode(torrents); err != nil {
		zap.L().Warn("JSON encode error", zap.Error(err))
	}
}

func apiTorrent(w http.ResponseWriter, r *http.Request) {
	infohashHex := mux.Vars(r)["infohash"]

	infohash, err := hex.DecodeString(infohashHex)
	if err != nil {
		respondError(w, 400, "couldn't decode infohash: %s", err.Error())
		return
	}

	torrent, err := database.GetTorrent(infohash)
	if err != nil {
		respondError(w, 500, "couldn't get torrent: %s", err.Error())
		return
	} else if torrent == nil {
		respondError(w, 404, "not found")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err = json.NewEncoder(w).Encode(torrent); err != nil {
		zap.L().Warn("JSON encode error", zap.Error(err))
	}
}

func apiFilelist(w http.ResponseWriter, r *http.Request) {
	infohashHex := mux.Vars(r)["infohash"]

	infohash, err := hex.DecodeString(infohashHex)
	if err != nil {
		respondError(w, 400, "couldn't decode infohash: %s", err.Error())
		return
	}

	files, err := database.GetFiles(infohash)
	if err != nil {
		respondError(w, 500, "couldn't get files: %s", err.Error())
		return
	} else if files == nil {
		respondError(w, 404, "not found")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err = json.NewEncoder(w).Encode(files); err != nil {
		zap.L().Warn("JSON encode error", zap.Error(err))
	}
}

func apiStatistics(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")

	// TODO: use gorilla?
	var n int64
	nStr := r.URL.Query().Get("n")
	if nStr == "" {
		n = 0
	} else {
		var err error
		n, err = strconv.ParseInt(nStr, 10, 32)
		if err != nil {
			respondError(w, 400, "couldn't parse n: %s", err.Error())
			return
		} else if n <= 0 {
			respondError(w, 400, "n must be a positive number")
			return
		}
	}

	stats, err := database.GetStatistics(from, uint(n))
	if err != nil {
		respondError(w, 400, "error while getting statistics: %s", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err = json.NewEncoder(w).Encode(stats); err != nil {
		zap.L().Warn("JSON encode error", zap.Error(err))
	}
}

func parseOrderBy(s string) (persistence.OrderingCriteria, error) {
	switch s {
	case "RELEVANCE":
		return persistence.ByRelevance, nil

	case "TOTAL_SIZE":
		return persistence.ByTotalSize, nil

	case "DISCOVERED_ON":
		return persistence.ByDiscoveredOn, nil

	case "N_FILES":
		return persistence.ByNFiles, nil

	case "UPDATED_ON":
		return persistence.ByUpdatedOn, nil

	case "N_SEEDERS":
		return persistence.ByNSeeders, nil

	case "N_LEECHERS":
		return persistence.ByNLeechers, nil

	default:
		return persistence.ByDiscoveredOn, fmt.Errorf("unknown orderBy string: %s", s)
	}
}
