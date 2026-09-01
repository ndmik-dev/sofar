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

  // E alone does nothing visible until a digit follows, which reads exactly
  // like a dead key. Say what the app is waiting for.
  function flash(text) {
    var slot = document.getElementById("toast");
    if (!slot) return;
    slot.innerHTML = "";
    var t = document.createElement("div");
    t.className = "toast";
    var s = document.createElement("span");
    s.textContent = text;
    t.appendChild(s);
    slot.appendChild(t);
  }

  function takeDigits() {
    var n = parseInt(digits, 10);
    digits = "";
    clearTimeout(digitTimer);
    return isNaN(n) ? 0 : n;
  }

  function statusOnly(row) {
    return !!row.querySelector(".stq");
  }

  function advance() {
    var row = selected();
    if (!row) return;
    // A status-depth row has no progress to advance; writing hidden progress
    // rows would quietly pollute the year statistics.
    if (statusOnly(row)) return;
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
    // Sourced on the slot so its hx-sync collapses reload storms: a newer
    // request replaces the one in flight instead of stacking behind it.
    window.htmx.ajax("GET", "/entry/" + row.dataset.entry + "/panel", {
      source: slot,
      target: "#panel-slot",
      swap: "innerHTML",
    });
  }

  // The panel refreshes by its own entry, not by the selected row: moving a
  // title to another list deletes that row, and then there was nothing left to
  // reload from and the panel kept showing the old state.
  window.reloadPanel = function () {
    var panel = document.getElementById("panel");
    var slot = document.getElementById("panel-slot");
    if (!panel || !slot) return;
    window.htmx.ajax("GET", "/entry/" + panel.dataset.entry + "/panel", {
      source: slot,
      target: "#panel-slot",
      swap: "innerHTML",
    });
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

  // Every palette action goes through one body-sourced request. Requests fired
  // from elements inside the open dialog lose their out-of-band swaps on some
  // pages, and this is the one path that provably never does.
  function paletteRequest(method, url, values) {
    return window.htmx.ajax(method, url, {
      source: document.body,
      target: "#palette-results",
      swap: "innerHTML",
      values: values,
    });
  }

  // The theme is a browser preference, not app data: it never leaves this
  // machine and there is nothing on the server to store it in.
  function applyTheme(v) {
    if (v === "dark" || v === "light") document.documentElement.dataset.theme = v;
    else delete document.documentElement.dataset.theme;
    var sw = document.getElementById("theme-switch");
    if (!sw) return;
    Array.prototype.forEach.call(sw.querySelectorAll("button"), function (b) {
      b.classList.toggle("on", b.dataset.themeSet === (v || "system"));
    });
  }

  function storedTheme() {
    try { return localStorage.getItem("sofar-theme") || "system"; } catch (e) { return "system"; }
  }

  document.addEventListener("click", function (e) {
    var t = e.target.closest("[data-theme-set]");
    if (t) {
      var v = t.dataset.themeSet;
      try {
        if (v === "system") localStorage.removeItem("sofar-theme");
        else localStorage.setItem("sofar-theme", v);
      } catch (err) {}
      applyTheme(v);
      return;
    }
  });

  document.addEventListener("DOMContentLoaded", function () { applyTheme(storedTheme()); });

  // Clicking a row selects it. Without this the keyboard kept acting on
  // whatever the arrows last touched, so Space after a click hit the wrong
  // title — the one bug that makes mouse and keyboard feel like two apps.
  document.addEventListener("click", function (e) {
    var row = e.target.closest(".rw[data-entry]");
    if (!row || row === selected()) return;
    select(row, false);
    if (panelOpen()) loadPanel();
  });

  // A track lets you click any episode to jump there. A bar had no way back
  // at all: shift-space stepped one page or one minute at a time.
  document.addEventListener("click", function (e) {
    var bar = e.target.closest(".bar.hit");
    if (!bar) return;
    var total = parseInt(bar.dataset.total, 10);
    var row = bar.closest("[data-entry]") || selected();
    if (!total || !row) return;

    var box = bar.getBoundingClientRect();
    if (!box.width) return;
    var share = (e.clientX - box.left) / box.width;
    var to = Math.round(Math.min(1, Math.max(0, share)) * total);

    window.htmx.ajax("POST", "/entry/" + row.dataset.entry + "/advance", {
      source: document.body,
      target: "#entry-" + row.dataset.entry,
      swap: "outerHTML",
      values: { abs: to },
    }).then(function () { window.reloadPanel(); });
  });

  document.addEventListener("click", function (e) {
    var el = e.target.closest("[data-pal]");
    if (!el || el.tagName === "FORM") return;
    var d = el.dataset;
    switch (d.pal) {
      case "add-tmdb":
        paletteRequest("POST", "/add", {
          tmdb_type: d.tmdbType,
          tmdb_id: d.tmdbId,
          status: window.sofarStatus(e),
        });
        break;
      case "add-manual":
        paletteRequest("POST", "/add/manual", {
          kind: d.kind, title: d.title, subtitle: d.subtitle,
          source: d.source, ext_id: d.extId, cover: d.cover, year: d.year,
          status: window.sofarStatus(e),
        });
        break;
      case "form":
        paletteRequest("GET", "/manual", {
          kind: d.kind, title: d.title, subtitle: d.subtitle,
          total: d.total, source: d.source, ext_id: d.extId, cover: d.cover,
        });
        break;
    }
  });

  document.addEventListener("submit", function (e) {
    var form = e.target.closest("[data-pal]");
    if (!form) return;
    e.preventDefault();
    var values = {};
    new FormData(form).forEach(function (v, k) { values[k] = v; });
    if (form.dataset.pal === "submit-manual") {
      values.status = window.sofarStatus(e);
      paletteRequest("POST", "/add/manual", values);
      return;
    }
    if (form.dataset.pal === "position") {
      window.htmx.ajax("POST", "/entry/" + form.dataset.entry + "/advance", {
        source: document.body,
        target: "#entry-" + form.dataset.entry,
        swap: "outerHTML",
        values: values,
      });
      window.closePalette();
    }
  });

  // Once a title is added the query that found it is noise; the field shows
  // what you are answering about instead.
  document.body.addEventListener("htmx:afterSettle", function () {
    var step = document.getElementById("position-step");
    var input = document.getElementById("palette-input");
    if (step && input && input.value !== step.dataset.title) {
      input.value = step.dataset.title;
    }
  });

  document.body.addEventListener("sofar:flow", function () {
    var input = document.getElementById("palette-input");
    var results = document.getElementById("palette-results");
    if (results) results.innerHTML = "";
    if (input) { input.value = ""; input.focus(); }
  });

  // Arriving from a shelf search: land on the row you were looking for rather
  // than at the top of a list that may be hundreds long.
  function focusFromURL() {
    var want = new URLSearchParams(location.search).get("focus");
    if (!want) return;
    var row = document.getElementById("entry-" + want);
    if (!row) return;
    select(row, false);
    row.scrollIntoView({ block: "center" });
  }

  // afterSettle, not afterSwap: htmx is still moving nodes during the swap
  // phase, and a highlight applied then is gone by the time it finishes.
  document.body.addEventListener("htmx:afterSettle", restore);
  document.addEventListener("DOMContentLoaded", function () { restore(); focusFromURL(); });
  restore();
  focusFromURL();

  document.addEventListener("keydown", function (e) {
    var mod = e.metaKey || e.ctrlKey;

    // Layout matters: on a Ukrainian keyboard ⌘K arrives as "к", not "k".
    // e.code is the physical key and settles the cases neither list catches.
    var key = e.key.length === 1 ? e.key.toLowerCase() : e.key;
    function is(latin, cyr1, cyr2) {
      return key === latin || key === cyr1 || key === cyr2 ||
             e.code === "Key" + latin.toUpperCase();
    }

    // Chrome keeps ⌘1–⌘9 for its own tabs and never hands them to the page, so
    // the pages are on ⌥. Matched by code, because ⌥1 types "¡" on macOS.
    if (!isTyping(e.target)) {
      var page = { Digit1: "/active", Digit2: "/backlog", Digit3: "/done", Digit4: "/year" };
      var to = (e.altKey && !mod && page[e.code]) || (mod && !e.shiftKey && !e.altKey && page["Digit" + e.key]);
      if (to) {
        e.preventDefault();
        window.location.href = to;
        return;
      }
    }

    if (mod && !e.altKey && is("c", "с", "с")) {
      var copySrc = document.getElementById("copy-year");
      if (copySrc && !isTyping(e.target) && !window.getSelection().toString()) {
        e.preventDefault();
        navigator.clipboard.writeText(copySrc.dataset.copy);
        copySrc.textContent = "скопійовано ✓";
        setTimeout(function () { copySrc.textContent = "⌘C копіювати текстом"; }, 1600);
        return;
      }
    }

    if (mod && !e.altKey && is("k", "к", "к")) {
      e.preventDefault();
      window.openPalette();
      return;
    }

    if (mod && !e.shiftKey && !e.altKey && is("z", "я", "з")) {
      if (isTyping(e.target)) return;
      var undo = document.querySelector("#toast .toast button");
      if (!undo) return;
      e.preventDefault();
      undo.click();
      return;
    }

    if (e.key === "Enter") {
      var palette = document.getElementById("palette");
      if (!palette || !palette.open) {
        if (isTyping(e.target) || !selected()) return;
        e.preventDefault();
        if (panelOpen()) window.closePanel(); else loadPanel();
        return;
      }
      // The search field always holds focus while the palette is open, so
      // "is the user typing" cannot gate this branch — only a real form can,
      // and only when the caret sits inside that form.
      var typedForm = isTyping(e.target) && e.target.closest("form[data-pal]");
      if (typedForm) {
        e.preventDefault();
        typedForm.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
        return;
      }
      var first = palette.querySelector(".res.on") || palette.querySelector(".res");
      if (!first) {
        var form = palette.querySelector("form[data-pal]");
        if (form) {
          e.preventDefault();
          pendingStatus = statusFromEvent(e);
          form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
        }
        return;
      }
      e.preventDefault();
      pendingStatus = statusFromEvent(e);
      first.click();
      return;
    }

    // The palette is a list you pick from, so it gets its own arrows. They are
    // handled before the dialog guard below, which stops every other shortcut.
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      var pal = document.getElementById("palette");
      if (pal && pal.open) {
        var items = Array.prototype.slice.call(pal.querySelectorAll(".res"));
        if (!items.length) return;
        e.preventDefault();
        var at = items.indexOf(pal.querySelector(".res.on"));
        var to;
        if (at < 0) {
          to = e.key === "ArrowDown" ? 0 : items.length - 1;
        } else {
          to = e.key === "ArrowDown" ? at + 1 : at - 1;
          to = Math.min(items.length - 1, Math.max(0, to));
        }
        items.forEach(function (el) { el.classList.remove("on"); });
        items[to].classList.add("on");
        items[to].scrollIntoView({ block: "nearest" });
        return;
      }
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
      if (e.key === "Escape") {
        e.preventDefault();
        flash("Оцінку скасовано");
        return;
      }
      // Any other key means you changed your mind: drop the state and let the
      // key do its own job rather than swallowing it.
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
          if (back && !statusOnly(back)) post("/entry/" + back.dataset.entry + "/advance", { n: -1 });
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

    if (is("s", "і", "с") && e.shiftKey) {
      e.preventDefault();
      finishSeason();
      return;
    }

    if (is("e", "у", "е") && selected()) {
      e.preventDefault();
      awaitingRating = true;
      flash("Оцінка: 1–9, 0 — це десять, Esc — скасувати");
      return;
    }

    // O opens where you watch it. A new tab, because losing the list to a
    // streaming site is not what the key is for.
    if (is("o", "щ", "о")) {
      var withLink = selected();
      if (withLink && withLink.dataset.link) {
        e.preventDefault();
        window.open(withLink.dataset.link, "_blank", "noopener");
        return;
      }
    }

    if (is("n", "т", "н") && panelOpen()) {
      var note = document.getElementById("panel-note");
      if (note) {
        e.preventDefault();
        // The note usually sits below the fold of a long panel, so focusing it
        // without scrolling looks exactly like the key doing nothing.
        note.scrollIntoView({ block: "center" });
        note.focus();
      }
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
