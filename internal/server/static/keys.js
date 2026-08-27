// Full keyboard layer lands in M6. Undo ships early because the toast already
// promises it, and a label that lies is worse than no label.
(function () {
  "use strict";

  function isTyping(el) {
    if (!el) return false;
    var tag = el.tagName;
    return tag === "INPUT" || tag === "TEXTAREA" || el.isContentEditable;
  }

  // Modifiers pick the status an added title lands in. A keyboard Enter has to
  // hand them over explicitly, because the synthetic click it fires carries none.
  var pendingStatus = null;

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

  window.closePalette = function () {
    var dialog = document.getElementById("palette");
    if (dialog && dialog.open) dialog.close();
  };

  function openPalette() {
    var dialog = document.getElementById("palette");
    if (!dialog || dialog.open) return;
    dialog.showModal();
    var input = document.getElementById("palette-input");
    if (input) {
      input.value = "";
      input.focus();
    }
    var results = document.getElementById("palette-results");
    if (results) results.innerHTML = "";
  }

  document.addEventListener("keydown", function (e) {
    if (!(e.metaKey || e.ctrlKey) || e.altKey) return;

    // Layouts matter: ⌘K on a Ukrainian keyboard reports "к", not "k".
    var key = e.key.toLowerCase();

    if (!e.shiftKey && (key === "k" || key === "к")) {
      e.preventDefault();
      openPalette();
      return;
    }

    if (!e.shiftKey && (key === "z" || key === "я")) {
      if (isTyping(e.target)) return;
      var undo = document.querySelector("#toast .toast button");
      if (!undo) return;
      e.preventDefault();
      undo.click();
    }
  });

  // Enter on the palette triggers the highlighted result, carrying whichever
  // modifier was held so the title lands in the right list.
  document.addEventListener("keydown", function (e) {
    if (e.key !== "Enter") return;
    var dialog = document.getElementById("palette");
    if (!dialog || !dialog.open) return;
    if (dialog.querySelector("form") && isTyping(e.target)) return;

    var first = dialog.querySelector(".res.on") || dialog.querySelector(".res");
    if (!first) return;
    e.preventDefault();
    pendingStatus = statusFromEvent(e);
    first.click();
  });

  // The toast fades out on a CSS timer. Drop the node too, otherwise the undo
  // button stays clickable long after it stopped being visible.
  document.addEventListener("animationend", function (e) {
    if (e.animationName === "toast-out" && e.target.classList.contains("toast")) {
      e.target.remove();
    }
  });
})();
