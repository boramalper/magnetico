// Derived from the Source: http://stackoverflow.com/a/17141374
// Copyright (c) 2013 'icktoofay' on stackoverflow.com
// Licensed under Creative Commons Attribution-ShareAlike 3.0 Unported (CC BY-SA 3.0)
// See https://creativecommons.org/licenses/by-sa/3.0/ for details of the license.
// See https://meta.stackexchange.com/questions/271080/the-mit-license-clarity-on-using-code-on-stack-overflow-and-stack-exchange
// for legal concerns "on using code on Stack Overflow and Stack Exchange".

"use strict";


window.onload = function() {
    let infoHash = window.location.pathname.split("/")[2];

    fetch("/api/v0.1/torrents/" + infoHash).then(x => x.json()).then(x => {
        document.querySelector("title").innerText = x.name + " - magneticow";

        const template = document.getElementById("main-template").innerHTML;
        document.querySelector("main").innerHTML = Mustache.render(template, {
            name:     x.name,
            infoHash: x.infoHash,
            sizeHumanised: fileSize(x.size),
            discoveredOnHumanised: humaniseDate(x.discoveredOn),
            nFiles: x.nFiles,
        });

        fetch("/api/v0.1/torrents/" + infoHash + "/filelist").then(x => x.json()).then(x => {
            const tree = new VanillaTree('#fileTree', {
                placeholder: 'Loading...',
            });

            for (let e of x) {
                let pathElems = e.path.split("/");

                for (let i = 0; i < pathElems.length; i++) {
                    tree.add({
                        id: pathElems.slice(0, i + 1).join("/"),
                        parent: i >= 1 ? pathElems.slice(0, i).join("/") : undefined,
                        label: pathElems[i] + ( i === pathElems.length - 1 ? "&emsp;<tt>" + fileSize(e.size) + "</tt>" : ""),
                        opened: true,
                    });
                }
            }
        });
    });
};
