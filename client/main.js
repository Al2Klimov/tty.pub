(function() {
    var clickable = true;

    document.getElementById("placeholder").onclick = function() {
        if (!clickable) {
            return;
        }

        clickable = false;

        var ws = new WebSocket(location.toString().replace(/^http/, "ws").replace(/\/$/, "") + "/v1");
        var placeholder = this;

        ws.onerror = function() {
            ws.onopen = null;
            clickable = true;
        };

        ws.onopen = function() {
            ws.onerror = null;
            placeholder.parentNode.removeChild(placeholder);

            var xterm = new Terminal;

            xterm.open(document.getElementById("terminal"));
            xterm.focus();

            xterm.onData(function(data) {
                if (ws.readyState === 1) {
                    ws.send(data);
                }
            });

            ws.onmessage = function(ev) {
                xterm.write(ev.data);
            };

            ws.onclose = function() {
                xterm.writeln("Connection to " + location.host + " closed.");
            };
        };
    };
})();
