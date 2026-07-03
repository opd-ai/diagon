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
├── reports.html          Index of generated diagonctl reports (links into reports/)
├── faq.html              Searchable FAQ (client-side filter)
├── about.html            Project mission, links, license, contributing
├── 404.html              Not-found page (uses absolute /diagon/ paths)
├── sitemap.xml           Search-engine sitemap
├── robots.txt            Crawler policy (allow all)
├── .nojekyll             Disables Jekyll processing
├── css/styles.css        "Field manual" theme (CSS variables, dark mode, print, a11y)
├── js/script.js          Progressive enhancement (menu, copy buttons, FAQ search)
└── reports/              Generated diagonctl artifacts (see "Regenerating reports")
```

## Regenerating reports

The files under `docs/reports/` are real outputs of `diagonctl`'s `--emit-*` flags,
generated for the `debian-12` environment. They are committed so the site can link to
them without a build step. When CLI output changes, regenerate them from the repo root:

```bash
go run ./cmd/diagonctl \
  --profile-dir profiles --profile-name myprofile \
  --policy-file profiles/validation-policy.json \
  --bootstrap-profile-file profiles/local-single-host-bootstrap.json \
  --service-contract-file profiles/service-contract.json \
  --integration-matrix-file .github/integration-matrix.json \
  --integration-environment debian-12 \
  --emit-operator-runbook-file docs/reports/operator-runbook.md \
  --emit-bootstrap-quickstart-file docs/reports/bootstrap-quickstart.md \
  --emit-wallet-validation-checklist-file docs/reports/wallet-validation-checklist.md \
  --emit-config-injection-file docs/reports/config-injection.json \
  --emit-debian-package-file docs/reports/debian-package.json \
  --emit-debian-dependency-manifest-file docs/reports/debian-dependency-manifest.json \
  --emit-debian-compose-bundle-file docs/reports/debian-compose-bundle.json \
  --emit-release-smoke-file docs/reports/release-smoke.json \
  --emit-release-baseline-file docs/reports/release-baseline.json \
  --emit-definition-of-done-file docs/reports/definition-of-done.json \
  --format json > docs/reports/validation-report.json
```

If you add a new emitter, add a `.report-item` card to `reports.html` and a line to the
command above.

## How to update the documentation

1. Edit the relevant file(s) under `docs/`.
2. Keep the shared top bar and left sidebar `<nav id="nav-links">` consistent across
   pages. Set `aria-current="page"` on the current page's sidebar link.
3. Use relative links between pages (for example `href="faq.html"`). The only file
   that uses absolute `/diagon/...` links is `404.html`, because a 404 response can be
   served from any URL depth.
4. If you add a page, also add it to:
   - the sidebar nav (`<aside class="sidebar">`) in every page,
   - the in-page `.page-footer` link lists where relevant,
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
