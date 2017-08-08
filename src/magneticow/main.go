// magneticow - Lightweight web interface for magnetico.
// Copyright (C) 2017  Mert Bora ALPER <bora@boramalper.org>
// Dedicated to Cemile Binay, in whose hands I thrived.
//
// This program is free software: you can redistribute it and/or modify  it under the terms of the
// GNU General Public License as published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without
// even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with this program.  If
// not, see <http://www.gnu.org/licenses/>.
package main

import (
	"net/http"

	"github.com/gorilla/mux"
)


func main() {
	router := mux.NewRouter()
	router.HandleFunc("/", rootHandler)
	router.HandleFunc("/torrents", torrentsHandler)
	router.HandleFunc("/torrents/{infohash}", torrentsInfohashHandler)
	router.HandleFunc("/torrents/{infohash}/{name}", torrentsInfohashNameHandler)
	router.HandleFunc("/statistics", statisticsHandler)
	router.HandleFunc("/feed", feedHandler)
	http.ListenAndServe(":8080", router)
}


func rootHandler(w http.ResponseWriter, r *http.Request) {

}


func torrentsHandler(w http.ResponseWriter, r *http.Request) {

}


func torrentsInfohashHandler(w http.ResponseWriter, r *http.Request) {

}


func torrentsInfohashNameHandler(w http.ResponseWriter, r *http.Request) {

}


func statisticsHandler(w http.ResponseWriter, r *http.Request) {

}


func feedHandler(w http.ResponseWriter, r *http.Request) {

}
