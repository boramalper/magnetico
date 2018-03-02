// Derived from the Source: http://stackoverflow.com/a/17141374
// Copyright (c) 2013 'icktoofay' on stackoverflow.com
// Licensed under Creative Commons Attribution-ShareAlike 3.0 Unported (CC BY-SA 3.0)
// See https://creativecommons.org/licenses/by-sa/3.0/ for details of the license.
// See https://meta.stackexchange.com/questions/271080/the-mit-license-clarity-on-using-code-on-stack-overflow-and-stack-exchange
// for legal concerns "on using code on Stack Overflow and Stack Exchange".

"use strict";


window.onload = function() {
    var pre_element = document.getElementsByTagName("pre")[0];
    var paths = pre_element.textContent.replace(/\s+$/, "").split("\n");
    paths = paths.map(function(path) { return path.split('/'); });
    pre_element.textContent = stringify(structurise(paths)).join("\n");
};


function structurise(paths) {
    var items = [];
    for(var i = 0, l = paths.length; i < l; i++) {
        var path = paths[i];
        var name = path[0];
        var rest = path.slice(1);
        var item = null;
        for(var j = 0, m = items.length; j < m; j++) {
            if(items[j].name === name) {
                item = items[j];
                break;
            }
        }
        if(item === null) {
            item = {name: name, children: []};
            items.push(item);
        }
        if(rest.length > 0) {
            item.children.push(rest);
        }
    }
    for(i = 0, l = items.length; i < l; i++) {
        item = items[i];
        item.children = structurise(item.children);
    }
    return items;
}


function stringify(items) {
    var lines = [];
    for(var i = 0, l = items.length; i < l; i++) {
        var item = items[i];
        lines.push(item.name);
        var subLines = stringify(item.children);
        for(var j = 0, m = subLines.length; j < m; j++) {
            lines.push("    " + subLines[j]);
        }
    }
    return lines;
}
