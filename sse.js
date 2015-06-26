"ues strict";

var RAND_ALPHABET = "0123456789abcdef"
function randId(bits) {
    var n = bits / Math.log2(RAND_ALPHABET.length);
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

function set(elementId, value) {
    var element = document.getElementById(elementId);
    if (element.value !== undefined) {
        element.value = value;
    } else {
        element.innerHTML = value;
    }
}
