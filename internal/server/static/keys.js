// Full keyboard layer lands in M6. Undo ships early because the toast already
// promises it, and a label that lies is worse than no label.
(function () {
  "use strict";

  function isTyping(el) {
    if (!el) return false;
    var tag = el.tagName;
    return tag === "INPUT" || tag === "TEXTAREA" || el.isContentEditable;
  }

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

  // Enter on the palette triggers the highlighted result.
  document.addEventListener("keydown", function (e) {
    if (e.key !== "Enter") return;
    var dialog = document.getElementById("palette");
    if (!dialog || !dialog.open) return;
    var first = dialog.querySelector(".res.on") || dialog.querySelector(".res");
    if (!first) return;
    e.preventDefault();
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
