"ues strict";

var RAND_ALPHABET = "0123456789abcdef"
function randId(bits) {
    var n = bits / Math.log(RAND_ALPHABET.length, 2);
    var chars = []
    for (var i = 0; i < n; i++) {
        var randIndex = Math.floor(Math.random() * RAND_ALPHABET.length);
        chars.push(RAND_ALPHABET[randIndex]);
    }
    return chars.join('');
}

var pageid = "page" + randId(128);

new EventSource('/events/' + pageid).onmessage = function(e) {
    console.log(e.data)
    eval(e.data);
}

function call(method /*, elements*/) {
    var data = [];
    Array.prototype.slice.call(arguments, 1).forEach(function (elementId) {
        data.push(get(elementId));
    });

    var r = new XMLHttpRequest();
    r.open('POST', '/call/' + pageid, true);
    r.setRequestHeader('Content-Type', 'application/json');
    r.send(JSON.stringify({'method': method, 'params': data}));
}

function get(elementId) {
    var element = document.getElementById(elementId);
    if (element.value !== undefined) {
        return element.value;
    } else {
        return element.innerHTML;
    }
}

var escapeMap = {
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
    "/": "&#x2F;",
};
var unescapeMap = {};
for (var key in escapeMap) { unescapeMap[escapeMap[key]] = key; }

function unescapeHtml(string) {
    return string.replace(/&.+?;/g, function (s) { return unescapeMap[s]; });
}
function escapeHtml(string) {
    return string.replace(/[&<>"'\/]/g, function (s) { return escapeMap[s]; });
}

function set(elementId, value) {
    var element = document.getElementById(elementId);
    if (element.value !== undefined) {
        element.value = value;
    } else {
        element.innerHTML = escapeHtml(value);
    }
}

function setRaw(elementId, value) {
    var element = document.getElementById(elementId);
    if (element.value !== undefined) {
        element.value = value;
    } else {
        element.innerHTML = value;
    }
}
