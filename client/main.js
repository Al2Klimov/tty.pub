document.getElementById("placeholder").onclick = function() {
    var xterm = new Terminal;

    this.parentNode.removeChild(this);
    xterm.open(document.getElementById("terminal"));
    xterm.focus();
};
