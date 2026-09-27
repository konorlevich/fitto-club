/* Fitto admin - progressive enhancement only. Every form posts without this
   file; here: theme switch without reload, error summary focus, sticky save
   bar state, "More" sheet closing, script detection for reviews, photo
   preview and focus point. Block comments only. */
(function () {
  "use strict";
  var d = document, root = d.documentElement;
  root.classList.add("js");

  /* Theme: apply immediately, then let the form POST persist it. */
  d.addEventListener("click", function (e) {
    var b = e.target.closest ? e.target.closest("[data-theme-form] button[name=theme]") : null;
    if (!b) { return; }
    var v = b.value;
    if (v === "auto") { delete root.dataset.theme; } else { root.dataset.theme = v; }
    b.closest("form").querySelectorAll("button[name=theme]").forEach(function (x) {
      if (x === b) { x.setAttribute("aria-pressed", "true"); } else { x.removeAttribute("aria-pressed"); }
    });
  });

  /* After a failed save the summary gets focus, so the screen reader and
     the eye land on what to fix. */
  var sum = d.getElementById("errsum");
  if (sum) { setTimeout(function () { sum.focus(); }, 0); }

  /* The "More" sheet: Esc closes, a tap outside closes, choosing a link closes. */
  var sheet = d.querySelector(".sheet");
  if (sheet) {
    d.addEventListener("keydown", function (e) { if (e.key === "Escape" && sheet.open) { sheet.open = false; sheet.querySelector("summary").focus(); } });
    d.addEventListener("click", function (e) { if (sheet.open && !sheet.contains(e.target)) { sheet.open = false; } });
    sheet.addEventListener("click", function (e) { if (e.target.closest(".morelist a")) { sheet.open = false; } });
  }

  /* Review language: guess the script while typing; the server does the
     same on save, so this is only a preview of the default. */
  var body = d.querySelector("[data-detect-script]");
  if (body) {
    var guess = function () {
      var t = body.value;
      var lang = /[Ⴀ-ჿᲐ-Ჿ]/.test(t) ? "ka" : /[Ѐ-ӿ]/.test(t) ? "ru" : /[A-Za-z]/.test(t) ? "en" : "";
      if (!lang) { return; }
      var r = d.querySelector('input[name=lang][value="' + lang + '"]');
      if (r && !d.querySelector("input[name=lang][data-user-set]")) { r.checked = true; }
    };
    body.addEventListener("input", guess);
    d.querySelectorAll("input[name=lang]").forEach(function (r) { r.addEventListener("change", function () { r.setAttribute("data-user-set", "1"); }); });
  }

  /* Photo: preview the chosen file before the upload round trip. */
  var file = d.querySelector("input[type=file][data-preview]");
  if (file) {
    file.addEventListener("change", function () {
      var f = file.files && file.files[0];
      var img = d.querySelector(file.getAttribute("data-preview"));
      if (!f || !img || !f.type.match(/^image\//)) { return; }
      var url = URL.createObjectURL(f);
      img.src = url; img.hidden = false;
      img.onload = function () { URL.revokeObjectURL(url); };
    });
  }

  /* Focus point: a tap on the preview moves the marker and fills the
     hidden fractions; the form still has to be saved. */
  var focus = d.querySelector("[data-focus]");
  if (focus) {
    var fx = d.querySelector("input[name=focus_x]"), fy = d.querySelector("input[name=focus_y]");
    var dot = focus.querySelector(".focus__dot");
    var radios = d.querySelectorAll("input[name=focus_preset]");
    var place = function (x, y) {
      x = Math.min(1, Math.max(0, x)); y = Math.min(1, Math.max(0, y));
      if (fx) { fx.value = x.toFixed(3); } if (fy) { fy.value = y.toFixed(3); }
      if (dot) { dot.style.left = (x * 100) + "%"; dot.style.top = (y * 100) + "%"; dot.hidden = false; }
      radios.forEach(function (r) { r.checked = false; });
    };
    focus.addEventListener("click", function (e) {
      var b = focus.getBoundingClientRect();
      place((e.clientX - b.left) / b.width, (e.clientY - b.top) / b.height);
    });
    focus.addEventListener("keydown", function (e) {
      var x = parseFloat(fx.value || "0.5"), y = parseFloat(fy.value || "0.5"), s = 0.05;
      if (e.key === "ArrowLeft") { place(x - s, y); } else if (e.key === "ArrowRight") { place(x + s, y); }
      else if (e.key === "ArrowUp") { place(x, y - s); } else if (e.key === "ArrowDown") { place(x, y + s); } else { return; }
      e.preventDefault();
    });
  }

  /* Highlight fades: remove the class after the token duration so a later
     print or screenshot is clean. */
  d.querySelectorAll(".is-new").forEach(function (el) { setTimeout(function () { el.classList.remove("is-new"); }, 2200); });
})();
/* Hours: the "day off" checkbox disables the pair of time inputs; the
   special-day "closed" switch hides its pair. Both are server-validated. */
(function () {
  "use strict";
  var d = document;
  d.querySelectorAll("[data-closed-for]").forEach(function (cb) {
    var n = cb.getAttribute("data-closed-for");
    var sync = function () {
      ["open_", "close_"].forEach(function (p) { var i = d.getElementById("f-" + p + n); if (i) { i.disabled = cb.checked; } });
    };
    cb.addEventListener("change", sync);
  });
  var closes = d.querySelector("[data-closes-hours]"), pair = d.querySelector("[data-hours-pair]");
  if (closes && pair) { closes.addEventListener("change", function () { pair.hidden = closes.checked; }); }
})();
/* Filters submit on change; the "Show" button stays for no-JS. */
(function () {
  "use strict";
  document.querySelectorAll("[data-autosubmit]").forEach(function (el) { el.addEventListener("change", function () { el.form.submit(); }); });
})();
