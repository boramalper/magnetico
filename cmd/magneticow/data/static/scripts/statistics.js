"use strict";

let nElem    = null,
    unitElem = null
;

window.onload = function() {
    nElem    = document.getElementById("n");
    unitElem = document.getElementById("unit");

    nElem.onchange = unitElem.onchange = load;

    load();
};

function plot(stats) {
    Plotly.newPlot("nDiscovered", [{
        x: Object.keys(stats.nDiscovered),
        y: Object.values(stats.nDiscovered),
        mode: "lines+markers"
    }], {
        title: "Torrents Discovered",
        xaxis: {
            title: "Date / Time",
        },
        yaxis: {
            title: "Number of Torrents Discovered",
        }
    });

    Plotly.newPlot("nFiles", [{
        x: Object.keys(stats.nFiles),
        y: Object.values(stats.nFiles),
        mode: "lines+markers"
    }], {
        title: "Files Discovered",
        xaxis: {
            title: "Date / Time",
        },
        yaxis: {
            title: "Number of Files Discovered",
        }
    });

    let totalSize = Object.values(stats.totalSize);
    for (let i in totalSize) {
        totalSize[i] = totalSize[i] / (1024 * 1024 * 1024);
    }

    Plotly.newPlot("totalSize", [{
        x: Object.keys(stats.totalSize),
        y: totalSize,
        mode: "lines+markers"
    }], {
        title: "Total Size of Files Discovered",
        xaxis: {
            title: "Date / Time",
        },
        yaxis: {
            title: "Total Size of Files Discovered (in TiB)",
        }
    });
}


function load() {
    const n = nElem.valueAsNumber;
    const unit = unitElem.options[unitElem.selectedIndex].value;

    const reqURL = "/api/v0.1/statistics?" + encodeQueryData({
        from: fromString(n, unit),
        n   : n,
    });
    console.log("reqURL", reqURL);

    let req = new XMLHttpRequest();
    req.onreadystatechange = function() {
        if (req.readyState !== 4)
            return;

        if (req.status !== 200)
            alert(req.responseText);

        let stats = JSON.parse(req.responseText);
        plot(stats);
    };

    req.open("GET", reqURL);
    req.send();
}


function fromString(n, unit) {
    const from = new Date(Date.now() - n * unit2seconds(unit) * 1000);
    console.log("frommmm", unit, unit2seconds(unit), from);

    let str = "" + from.getUTCFullYear();
    if (unit === "years")
        return str;
    else if (unit === "weeks") {
        str += "-W" + leftpad(from.getWeek());
        return str;
    } else {
        str += "-" + leftpad(from.getUTCMonth() + 1);
        if (unit === "months")
            return str;

        str += "-" + leftpad(from.getUTCDate());
        if (unit === "days")
            return str;

        str += "T" + leftpad(from.getUTCHours());
        if (unit === "hours")
            return str;
    }

    return str;


    function unit2seconds(u) {
        if (u === "hours")  return            60 * 60;
        if (u === "days")   return       24 * 60 * 60;
        if (u === "weeks")  return   7 * 24 * 60 * 60;
        if (u === "months") return  30 * 24 * 60 * 60;
        if (u === "years")  return 365 * 24 * 60 * 60;
    }


    // pad x to minimum of n characters with c
    function leftpad(x, n, c) {
        if (n === undefined)
            n = 2;
        if (c === undefined)
            c = "0";

        const xs = "" + x;
        if (n > xs.length)
            return c.repeat(n - xs.length) + x;
        else
            return x;
    }
}
