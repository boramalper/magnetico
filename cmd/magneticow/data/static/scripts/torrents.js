"use strict";

const query = (new URL(location)).searchParams.get("query")
    , epoch = Math.floor(Date.now() / 1000)
;
let orderBy, ascending;  // use `setOrderBy()` to modify orderBy
let lastOrderedValue, lastID;


window.onload = function() {
    if (query !== null && query !== "") {
        orderBy = "RELEVANCE";
    }

    const title = document.getElementsByTagName("title")[0];
    if (query) {
        title.textContent = query + " - magneticow";
        const input = document.getElementsByTagName("input")[0];
        input.setAttribute("value", query);

        setOrderBy("RELEVANCE");
    } else {
        title.textContent = "Most recent torrents - magneticow";

        ascending = false;
        setOrderBy("DISCOVERED_ON");
    }

    if (query) {
        const feedAnchor = document.getElementById("feed-anchor");
        feedAnchor.setAttribute("href", "/feed?query=" + encodeURIComponent(query));
    }

    load();
};


function setOrderBy(x) {
    const validValues = [
        "TOTAL_SIZE",
        "DISCOVERED_ON",
        "UPDATED_ON",
        "N_FILES",
        "N_SEEDERS",
        "N_LEECHERS",
        "RELEVANCE"
    ];
    if (!validValues.includes(x)) {
        throw new Error("invalid value for @orderBy");
    }
    orderBy = x;
}

function orderedValue(torrent) {
    if      (orderBy === "TOTAL_SIZE")    return torrent.size;
    else if (orderBy === "DISCOVERED_ON") return torrent.discoveredOn;
    else if (orderBy === "UPDATED_ON")    alert("implement it server side first!");
    else if (orderBy === "N_FILES")       return torrent.nFiles;
    else if (orderBy === "N_SEEDERS")     alert("implement it server side first!");
    else if (orderBy === "N_LEECHERS")    alert("implement it server side first!");
    else if (orderBy === "RELEVANCE")     return torrent.relevance;
}


function load() {
    const button   = document.getElementsByTagName("button")[0];
    button.textContent = "Loading More Results...";
    button.setAttribute("disabled", "");  // disable the button whilst loading...

    const ul       = document.querySelector("main ul");
    const template = document.getElementById("item-template").innerHTML;
    const reqURL   = "/api/v0.1/torrents?" + encodeQueryData({
        query           : query,
        epoch           : epoch,
        lastID          : lastID,
        lastOrderedValue: lastOrderedValue,
        orderBy         : orderBy,
        ascending       : ascending
    });

    console.log("reqURL", reqURL);

    let req = new XMLHttpRequest();
    req.onreadystatechange = function() {
        if (req.readyState !== 4)
            return;

        button.textContent = "Load More Results";
        button.removeAttribute("disabled");

        if (req.status !== 200)
            alert(req.responseText);

        let torrents = JSON.parse(req.responseText);
        if (torrents.length === 0) {
            button.textContent = "No More Results";
            button.setAttribute("disabled", "");
            return;
        }

        const last = torrents[torrents.length - 1];
        lastID           = last.id;
        lastOrderedValue = orderedValue(last);

        for (let t of torrents) {
            t.size = fileSize(t.size);
            t.discoveredOn = humaniseDate(t.discoveredOn);

            ul.innerHTML += Mustache.render(template, t);
        }
    };

    req.open("GET", reqURL);
    req.send();
}
