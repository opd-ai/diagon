# Documentation site maintenance

The diagon end-user documentation site is published with **GitHub Pages** from the
[`docs/`](../docs/) folder on the `main` branch.

- **Live URL:** https://opd-ai.github.io/diagon/
- **Source:** `main` branch, `/docs` folder
- **Build:** none — plain static HTML/CSS/JS, no framework, no external dependencies
- **Jekyll:** disabled via [`docs/.nojekyll`](../docs/.nojekyll) so files are served as-is

## Site structure

```
docs/
├── index.html            Landing page (introduction, features, quick links)
├── getting-started.html  Quick start (prerequisites, validation, first run)
├── installation.html     Detailed install (Debian, OpenBSD, compose fallback)
├── configuration.html    Config reference (profiles, contracts, secrets, tuning)
├── variants.html         Variant/edition comparison
├── faq.html              Searchable FAQ (client-side filter)
├── about.html            Project mission, links, license, contributing
├── 404.html              Not-found page (uses absolute /diagon/ paths)
├── sitemap.xml           Search-engine sitemap
├── robots.txt            Crawler policy (allow all)
├── .nojekyll             Disables Jekyll processing
├── css/styles.css        Shared styles (CSS variables, dark mode, print, a11y)
└── js/script.js          Progressive enhancement (menu, copy buttons, FAQ search)
```

## How to update the documentation

1. Edit the relevant file(s) under `docs/`.
2. Keep the shared header `<nav>` and footer consistent across pages. Set
   `aria-current="page"` on the current page's nav link.
3. Use relative links between pages (for example `href="faq.html"`). The only file
   that uses absolute `/diagon/...` links is `404.html`, because a 404 response can be
   served from any URL depth.
4. If you add a page, also add it to:
   - the primary nav in every page's `<header>`,
   - the footer link lists,
   - `sitemap.xml`.
5. Commit and push to `main`. GitHub Pages redeploys automatically (typically under a minute).

## Content sources

All content is derived from the repository itself — primarily
[`ROADMAP.md`](../ROADMAP.md) and [`README.md`](../README.md). When project scope,
CLI flags, variants, or CI stages change, update the corresponding page so the site
stays accurate.

## Preview before publishing

Preview locally with any static file server from the repository root:

```bash
python3 -m http.server 8080 --directory docs
# then open http://localhost:8080/
```

Because there is no build step, what you see locally is exactly what Pages serves.

## Design and accessibility guidelines

- **Minimalist and dependency-free.** No CSS/JS frameworks, no third-party requests.
- **Accessibility (WCAG 2.1 AA).** Maintain a single `<h1>` per page and a logical
  heading order, keep a visible skip link, preserve focus-visible outlines, provide
  text alternatives, and keep color contrast high in both light and dark themes.
- **Responsive.** Mobile-first layout; verify the mobile menu toggle still works if you
  change the header.
- **Performance.** Keep pages small and avoid adding external assets.

## Maintenance schedule

- Review the site whenever `ROADMAP.md`, CLI flags, or supported variants change.
- Do a light accuracy pass each release to confirm commands and version references are current.

## CI note

The documentation lives on `main` and does **not** define any GitHub Actions workflow,
so it does not add or alter CI jobs. The existing `build.yml` and `openbsd.yml`
pipelines are unaffected by `docs/` changes.
