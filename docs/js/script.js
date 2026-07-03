/* diagon docs — minimal client-side enhancements. No dependencies. */
(function () {
  "use strict";

  var toggle = document.querySelector(".menu-toggle");
  var nav = document.getElementById("nav-links");
  if (toggle && nav) {
    toggle.addEventListener("click", function () {
      var open = nav.classList.toggle("open");
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
    });
    nav.addEventListener("click", function (e) {
      if (e.target.tagName === "A" && window.matchMedia("(max-width: 860px)").matches) {
        nav.classList.remove("open");
        toggle.setAttribute("aria-expanded", "false");
      }
    });
  }

  // Copy buttons for code blocks
  document.querySelectorAll("pre").forEach(function (pre) {
    var code = pre.querySelector("code");
    if (!code) return;
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "copy-btn";
    btn.textContent = "copy";
    btn.setAttribute("aria-label", "Copy code to clipboard");
    btn.addEventListener("click", function () {
      var text = code.innerText;
      var done = function () {
        btn.textContent = "copied";
        setTimeout(function () { btn.textContent = "copy"; }, 1500);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done, function () { fallback(text, done); });
      } else { fallback(text, done); }
    });
    pre.appendChild(btn);
  });

  function fallback(text, done) {
    var ta = document.createElement("textarea");
    ta.value = text; ta.style.position = "fixed"; ta.style.opacity = "0";
    document.body.appendChild(ta); ta.select();
    try { document.execCommand("copy"); done(); } catch (e) {}
    document.body.removeChild(ta);
  }

  // FAQ client-side filter
  var search = document.getElementById("faq-search-input");
  if (search) {
    var items = Array.prototype.slice.call(document.querySelectorAll(".faq-item"));
    var categories = Array.prototype.slice.call(document.querySelectorAll(".faq-category"));
    var empty = document.getElementById("faq-empty");
    search.addEventListener("input", function () {
      var q = search.value.trim().toLowerCase();
      var anyVisible = false;
      items.forEach(function (item) {
        var match = q === "" || item.textContent.toLowerCase().indexOf(q) !== -1;
        item.style.display = match ? "" : "none";
        if (match) anyVisible = true;
        if (q !== "" && match) { item.setAttribute("open", ""); }
        else if (q === "") { item.removeAttribute("open"); }
      });
      categories.forEach(function (cat) {
        var visible = cat.querySelectorAll('.faq-item:not([style*="display: none"])');
        cat.style.display = visible.length ? "" : "none";
      });
      if (empty) empty.hidden = anyVisible;
    });
  }
})();
