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
    }
    else
        title.textContent = "Most recent torrents - magneticow";

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

    const tbody    = document.getElementsByTagName("tbody")[0];
    const template = document.getElementById("row-template").innerHTML;
    const reqURL   = "/api/v0.1/torrents?" + encodeQueryData({
        query           : query,
        epoch           : epoch,
        lastID          : lastID,
        lastOrderedValue: lastOrderedValue
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

        for (let t of torrents) {
            t.size = fileSize(t.size);
            t.discoveredOn = (new Date(t.discoveredOn * 1000)).toLocaleDateString("en-GB", {
                day: "2-digit",
                month: "2-digit",
                year: "numeric"
            });

            tbody.innerHTML += Mustache.render(template, t);
        }

        const last = torrents[torrents.length - 1];
        lastID           = last.id;
        lastOrderedValue = orderedValue(last);
    };

    req.open("GET", reqURL);
    req.send();
}


// Source: https://stackoverflow.com/a/111545/4466589
function encodeQueryData(data) {
    let ret = [];
    for (let d in data) {
        if (data[d] === null || data[d] === undefined)
            continue;
        ret.push(encodeURIComponent(d) + "=" + encodeURIComponent(data[d]));
    }
    return ret.join("&");
}


// https://stackoverflow.com/q/10420352/4466589
function fileSize(fileSizeInBytes) {
    let i = -1;
    let byteUnits = [' KiB', ' MiB', ' GiB', ' TiB', ' PiB', ' EiB', ' ZiB', ' YiB'];
    do {
        fileSizeInBytes = fileSizeInBytes / 1024;
        i++;
    } while (fileSizeInBytes > 1024);

    return Math.max(fileSizeInBytes, 0.1).toFixed(1) + byteUnits[i];
}
