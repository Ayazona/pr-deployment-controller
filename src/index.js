import { Terminal } from "xterm";
import * as fit from "xterm/lib/addons/fit/fit";
import * as fullscreen from "xterm/lib/addons/fullscreen/fullscreen";

import "xterm/dist/xterm.css";
import "xterm/dist/addons/fullscreen/fullscreen.css";

document.addEventListener("DOMContentLoaded", () => {
  // Apply addons
  Terminal.applyAddon(fit);
  Terminal.applyAddon(fullscreen);

  // Initialize term
  var term;

  // Initialize websocket
  var websocket = new WebSocket(
    "wss://" +
      window.location.hostname +
      ":" +
      window.location.port +
      window.location.pathname +
      "ws/"
  );
  websocket.binaryType = "arraybuffer";

  function ab2str(buf) {
    return String.fromCharCode.apply(null, new Uint8Array(buf));
  }

  websocket.onopen = function(evt) {
    // Create term on connect
    term = new Terminal({
      screenKeys: true,
      useStyle: true,
      cursorBlink: true
    });

    // Set term on window for debugging purposes
    window.term = term;

    // Send data to the websocket
    term.on("data", function(data) {
      websocket.send(new TextEncoder().encode("\x00" + data));
    });

    // Send resize events to the websocket
    term.on("resize", function(evt) {
      websocket.send(
        new TextEncoder().encode(
          "\x01" + JSON.stringify({ cols: evt.cols, rows: evt.rows })
        )
      );
    });

    // Update document title
    term.on("title", function(title) {
      document.title = title;
    });

    // Open and fit terminal
    term.open(document.getElementById("terminal"));
    term.fit();
    term.toggleFullScreen(true);

    // Write input data to the terminal
    websocket.onmessage = function(evt) {
      if (evt.data instanceof ArrayBuffer) {
        term.write(ab2str(evt.data));
      } else {
        alert(evt.data);
      }
    };

    // Write session closed to the terminal when the session closes
    websocket.onclose = function(evt) {
      term.write("Session terminated");
      term.destroy();
    };

    websocket.onerror = function(evt) {
      if (typeof console.log == "function") {
        console.log(evt);
      }
    };

    // Fit the terminal each time the window size changes
    window.onresize = function(event) {
      term.fit();
    };
  };
});
