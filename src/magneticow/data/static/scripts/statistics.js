"use strict";


window.onload = function() {
    Plotly.newPlot("discoveryRateGraph", discoveryRateData, {
        title: "New Discovered Torrents Per Day in the Past 30 Days",
        xaxis: {
            title: "Days",
            tickformat: "%d %B"
        },
        yaxis: {
            title: "Amount of Torrents Discovered"
        }
    });
};
