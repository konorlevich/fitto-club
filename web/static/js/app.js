/* Fitto Club - the whole JS budget, inlined. Progressive enhancement only:
   every link, the menu, the schedule and the video work with this blocked.
   Block comments only - the boot minifier strips them and indentation. */
(function () {
  "use strict";
  var d = document, root = d.documentElement;
  root.classList.add("js");

  /* Storage access throws in some private modes; never let it kill the rest. */
  function read(k) { try { return localStorage.getItem(k); } catch (e) { return null; } }
  function write(k, v) { try { localStorage.setItem(k, v); } catch (e) {} }

  /* Analytics: one helper that branches on the tag kind the server resolved.
     No tag configured is a silent no-op. Events come from data-* attributes
     in the templates, never from CSS selectors. */
  var tag = window.__tag || { kind: "" };
  function track(name, params) {
    try {
      if (tag.kind === "gtm") {
        var p = { event: name };
        for (var k in params) { if (Object.prototype.hasOwnProperty.call(params, k)) { p[k] = params[k]; } }
        (window.dataLayer = window.dataLayer || []).push(p);
      } else if (tag.kind === "ga4" && typeof window.gtag === "function") {
        window.gtag("event", name, params);
      }
    } catch (e) {}
  }
  d.addEventListener("click", function (e) {
    var a = e.target.closest ? e.target.closest("a,button") : null;
    if (!a) { return; }
    var loc = a.getAttribute("data-loc") || "";
    if (a.hasAttribute("data-contact")) {
      track("contact_click", { method: a.getAttribute("data-contact"), location: loc });
    } else if (a.getAttribute("data-cta") === "directions") {
      track("select_content", { content_type: "cta", cta_label: "directions", location: loc });
    } else if (a.hasAttribute("data-lang")) {
      track("language_switch", { language: a.getAttribute("data-lang"), from_language: root.lang });
    }
  });
  if (d.querySelector(".nf")) { track("page_not_found", { page_path: location.pathname }); }

  /* Consent banner: non-modal, never traps focus. */
  var consent = d.querySelector(".consent");
  function setConsent(granted) {
    write("consent", JSON.stringify({ v: 1, granted: granted, t: Date.now() }));
    if (typeof window.gtag === "function") { window.gtag("consent", "update", { analytics_storage: granted ? "granted" : "denied" }); }
    if (consent) { consent.hidden = true; }
  }
  if (consent) {
    var stored = null;
    try { stored = JSON.parse(read("consent") || "null"); } catch (e) {}
    if (!stored || stored.v !== 1 || (Date.now() - stored.t) > 15552000000) { consent.hidden = false; }
    consent.addEventListener("click", function (e) {
      var b = e.target.closest("[data-consent]");
      if (b) { setConsent(b.getAttribute("data-consent") === "accept"); }
    });
    d.addEventListener("click", function (e) {
      if (e.target.closest("[data-consent-open]")) { consent.hidden = false; consent.querySelector("button").focus(); }
    });
  }

  /* Menu: Esc closes and returns focus, choosing a link closes, the page
     under the open panel does not scroll. */
  var menu = d.querySelector(".menu");
  if (menu) {
    var btn = menu.querySelector("summary");
    menu.addEventListener("toggle", function () { root.style.overflow = menu.open ? "hidden" : ""; });
    d.addEventListener("keydown", function (e) {
      if (e.key === "Escape" && menu.open) { menu.open = false; btn.focus(); }
    });
    menu.addEventListener("click", function (e) { if (e.target.closest(".menu__panel a")) { menu.open = false; } });
    window.matchMedia("(min-width: 1024px)").addEventListener("change", function (m) { if (m.matches) { menu.open = false; } });
  }

  /* Anchor jumps move focus to the target heading, so keyboard and screen
     reader users land where the eye does. */
  d.addEventListener("click", function (e) {
    var a = e.target.closest ? e.target.closest('a[href^="#"]') : null;
    if (!a) { return; }
    var id = a.getAttribute("href").slice(1);
    var target = id && d.getElementById(id);
    if (!target) { return; }
    var focusEl = target.matches("h1,h2,h3,[tabindex]") ? target : target.querySelector("h2[tabindex],h2,h1");
    if (focusEl) {
      if (!focusEl.hasAttribute("tabindex")) { focusEl.setAttribute("tabindex", "-1"); }
      setTimeout(function () { focusEl.focus({ preventScroll: true }); }, 0);
    }
  });

  /* Sticky CTA bar: shown once the hero has scrolled away, hidden while the
     "Getting here" block is on screen. */
  var bar = d.querySelector(".ctabar");
  if (bar && "IntersectionObserver" in window) {
    var hero = d.querySelector(".hero, .page-head, main h1");
    var visit = d.getElementById("visit");
    var heroGone = !hero, visitOn = false;
    bar.hidden = false;
    d.body.classList.add("has-bar");
    function sync() { bar.classList.toggle("is-off", !heroGone || visitOn); bar.inert = !heroGone || visitOn; }
    sync();
    if (hero) {
      new IntersectionObserver(function (es) { heroGone = !es[0].isIntersecting; sync(); }).observe(hero);
    }
    if (visit) {
      new IntersectionObserver(function (es) { visitOn = es[0].isIntersecting; sync(); }, { rootMargin: "0px 0px -30% 0px" }).observe(visit);
    }
  }

  /* Entrance video: the poster is a link to the file; with JS it becomes an
     inline player, loaded only on click. */
  d.addEventListener("click", function (e) {
    var play = e.target.closest ? e.target.closest("[data-play]") : null;
    if (!play) { return; }
    e.preventDefault();
    var box = play.closest("[data-video]");
    var v = d.createElement("video");
    v.src = box.getAttribute("data-video");
    v.controls = true; v.playsInline = true; v.muted = true; v.preload = "auto";
    v.setAttribute("aria-label", play.textContent.trim());
    box.replaceChildren(v);
    v.play().catch(function () {});
    v.focus();
  });

  /* The bouncing word hops again on hover or focus of the hero. */
  var word = d.querySelector(".bounce");
  if (word && !window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
    var again = function () {
      word.classList.remove("is-replay");
      void word.offsetWidth;
      word.classList.add("is-replay");
    };
    word.parentNode.addEventListener("mouseenter", again);
  }

  /* Fade-up on scroll. Content is fully present without JS; with reduced
     motion the CSS keeps it static. */
  if ("IntersectionObserver" in window) {
    var io = new IntersectionObserver(function (es) {
      es.forEach(function (en) { if (en.isIntersecting) { en.target.classList.add("is-in"); io.unobserve(en.target); } });
    }, { rootMargin: "0px 0px -8% 0px" });
    /* Read every position first, then write: interleaving the two forced a
       layout per element (seen in the performance trace). */
    var els = Array.prototype.slice.call(d.querySelectorAll(".section__head, .steps__item, .coach-card, .zone, .plan, .review, .class, .extra"));
    var h = window.innerHeight;
    var below = els.filter(function (el) { return el.getBoundingClientRect().top > h; });
    below.forEach(function (el) { el.classList.add("reveal"); io.observe(el); });
  }
})();
