(function () {
  "use strict";

  var DIGIT_WINDOW = 1500;

  var selectedId = null;
  var digits = "";
  var digitTimer = null;
  var awaitingRating = false;
  var pendingStatus = null;

  function isTyping(el) {
    if (!el) return false;
    var tag = el.tagName;
    return tag === "INPUT" || tag === "TEXTAREA" || el.isContentEditable;
  }

  function dialogOpen() {
    return !!document.querySelector("dialog[open]");
  }

  function rows() {
    return Array.prototype.slice.call(document.querySelectorAll(".rw[data-entry]"));
  }

  function selected() {
    return selectedId ? document.getElementById("entry-" + selectedId) : null;
  }

  function select(row, scroll) {
    if (!row) return;
    rows().forEach(function (r) { r.classList.remove("on"); });
    row.classList.add("on");
    selectedId = row.dataset.entry;
    if (scroll) row.scrollIntoView({ block: "nearest" });
  }

  function move(step) {
    var all = rows();
    if (!all.length) return;
    var i = all.indexOf(selected());
    if (i < 0) {
      select(all[step > 0 ? 0 : all.length - 1], true);
      return;
    }
    select(all[Math.min(all.length - 1, Math.max(0, i + step))], true);
    if (panelOpen()) loadPanel();
  }

  // Selection lives in the client, but htmx replaces rows underneath it, so it
  // has to be re-applied after every swap or the highlight jumps home.
  function restore() {
    var all = rows();
    if (!all.length) { selectedId = null; return; }
    select(selected() || all[0], false);
  }

  // Every keyboard action lands here, so this is the one place that keeps an
  // open panel in sync with the row it describes.
  function post(url, body) {
    var row = selected();
    if (!row) return;
    window.htmx.ajax("POST", url, {
      target: "#entry-" + row.dataset.entry,
      swap: "outerHTML",
      values: body || {},
    }).then(function () {
      window.reloadPanel();
    });
  }

  function takeDigits() {
    var n = parseInt(digits, 10);
    digits = "";
    clearTimeout(digitTimer);
    return isNaN(n) ? 0 : n;
  }

  function advance() {
    var row = selected();
    if (!row) return;
    var typed = takeDigits();
    var step = typed > 0 ? typed : parseInt(row.dataset.step, 10) || 1;
    post("/entry/" + row.dataset.entry + "/advance", { n: step });
  }

  function finishSeason() {
    var row = selected();
    if (!row) return;
    var end = parseInt(row.dataset.seasonEnd, 10) || 0;
    if (end) post("/entry/" + row.dataset.entry + "/advance", { abs: end });
  }

  function statusFromEvent(e) {
    if (!e) return "active";
    if (e.shiftKey) return "backlog";
    if (e.metaKey || e.ctrlKey) return "done";
    if (e.altKey) return "dropped";
    return "active";
  }

  window.sofarStatus = function (e) {
    if (pendingStatus) {
      var s = pendingStatus;
      pendingStatus = null;
      return s;
    }
    return statusFromEvent(e);
  };

  function panelOpen() {
    return !!document.getElementById("panel");
  }

  window.closePanel = function () {
    var slot = document.getElementById("panel-slot");
    if (slot) slot.innerHTML = "";
  };

  // The panel follows the cursor: opening it and moving the selection are the
  // same action, so there is never a panel showing a row you are not on.
  function loadPanel() {
    var row = selected();
    var slot = document.getElementById("panel-slot");
    if (!row || !slot) return;
    window.htmx.ajax("GET", "/entry/" + row.dataset.entry + "/panel", {
      target: "#panel-slot",
      swap: "innerHTML",
    });
  }

  window.reloadPanel = function () {
    if (panelOpen()) loadPanel();
  };

  window.closePalette = function () {
    var d = document.getElementById("palette");
    if (d && d.open) d.close();
  };

  window.openPalette = function () {
    var d = document.getElementById("palette");
    if (!d || d.open) return;
    d.showModal();
    var input = document.getElementById("palette-input");
    if (input) { input.value = ""; input.focus(); }
    ["palette-results", "flow-log"].forEach(function (id) {
      var el = document.getElementById(id);
      if (el) el.innerHTML = "";
    });
  };

  document.body.addEventListener("sofar:flow", function () {
    var input = document.getElementById("palette-input");
    var results = document.getElementById("palette-results");
    if (results) results.innerHTML = "";
    if (input) { input.value = ""; input.focus(); }
  });

  // afterSettle, not afterSwap: htmx is still moving nodes during the swap
  // phase, and a highlight applied then is gone by the time it finishes.
  document.body.addEventListener("htmx:afterSettle", restore);
  document.addEventListener("DOMContentLoaded", restore);
  restore();

  document.addEventListener("keydown", function (e) {
    var mod = e.metaKey || e.ctrlKey;

    // Layout matters: on a Ukrainian keyboard ⌘K arrives as "к", not "k".
    var key = e.key.length === 1 ? e.key.toLowerCase() : e.key;

    if (mod && !e.altKey && (key === "k" || key === "к")) {
      e.preventDefault();
      window.openPalette();
      return;
    }

    if (mod && !e.shiftKey && !e.altKey && (key === "z" || key === "я")) {
      if (isTyping(e.target)) return;
      var undo = document.querySelector("#toast .toast button");
      if (!undo) return;
      e.preventDefault();
      undo.click();
      return;
    }

    if (e.key === "Enter" && !isTyping(e.target)) {
      var palette = document.getElementById("palette");
      if (!palette || !palette.open) {
        if (!selected()) return;
        e.preventDefault();
        if (panelOpen()) window.closePanel(); else loadPanel();
        return;
      }
      if (palette.querySelector("form") && isTyping(e.target)) return;
      var first = palette.querySelector(".res.on") || palette.querySelector(".res");
      if (!first) return;
      e.preventDefault();
      pendingStatus = statusFromEvent(e);
      first.click();
      return;
    }

    if (e.key === "Escape" && panelOpen() && !dialogOpen()) {
      e.preventDefault();
      window.closePanel();
      if (isTyping(e.target)) e.target.blur();
      return;
    }

    if (isTyping(e.target) || dialogOpen()) return;

    if (e.key === "?") {
      var help = document.getElementById("help");
      if (help && !help.open) { e.preventDefault(); help.showModal(); }
      return;
    }

    // E waits for one digit, so 9 rates and 0 means ten.
    if (awaitingRating) {
      awaitingRating = false;
      if (/^[0-9]$/.test(e.key)) {
        e.preventDefault();
        var rated = selected();
        if (rated) {
          post("/entry/" + rated.dataset.entry + "/rating", {
            rating: e.key === "0" ? 10 : Number(e.key),
          });
        }
        return;
      }
      if (e.key === "Escape") { e.preventDefault(); return; }
    }

    if (mod && e.key === "Backspace") {
      e.preventDefault();
      var drop = selected();
      if (drop) post("/entry/" + drop.dataset.entry + "/status", { status: "dropped" });
      return;
    }

    if (mod || e.altKey) return;

    switch (e.key) {
      case "ArrowDown":
        e.preventDefault(); move(1); return;
      case "ArrowUp":
        e.preventDefault(); move(-1); return;
      case " ":
        e.preventDefault();
        if (e.shiftKey) {
          var back = selected();
          takeDigits();
          if (back) post("/entry/" + back.dataset.entry + "/advance", { n: -1 });
        } else {
          advance();
        }
        return;
    }

    if (/^[0-9]$/.test(e.key)) {
      e.preventDefault();
      digits += e.key;
      clearTimeout(digitTimer);
      digitTimer = setTimeout(function () { digits = ""; }, DIGIT_WINDOW);
      return;
    }

    if ((key === "s" || key === "і") && e.shiftKey) {
      e.preventDefault();
      finishSeason();
      return;
    }

    if ((key === "e" || key === "у") && selected()) {
      e.preventDefault();
      awaitingRating = true;
      return;
    }

    if ((key === "n" || key === "т") && panelOpen()) {
      var note = document.getElementById("panel-note");
      if (note) { e.preventDefault(); note.focus(); }
    }
  });

  // The toast fades out on a CSS timer. Drop the node too, otherwise the undo
  // button stays clickable long after it stopped being visible.
  document.addEventListener("animationend", function (e) {
    if (e.animationName === "toast-out" && e.target.classList.contains("toast")) {
      e.target.remove();
    }
  });
})();
