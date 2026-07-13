package main

import (
	"fmt"
	"html"
	"strings"
)

// pageData is everything renderPage needs for one page.
type pageData struct {
	version  string // this page's version label
	rel      string // this page's POSIX path within the version dir
	title    string // page title (also drives <title>)
	bodyHTML string // rendered Markdown body
	docs     []doc  // all docs in this version (for the nav)
	manifest manifest
}

// dirDepth is the number of path segments the page sits below the version root.
func dirDepth(rel string) int {
	return strings.Count(rel, "/")
}

// renderPage assembles a full standalone HTML document: shared head/CSS, a
// left nav, a version-selector dropdown, and the rendered body.
//
// Path bookkeeping (all relative, so the site works under any base path such as
// GitHub Pages' /vendkit/):
//   - versionPrefix reaches the version root from this page ("" or "../"…).
//   - rootPrefix reaches the site root (where versions.json lives).
func renderPage(p pageData) string {
	depth := dirDepth(p.rel)
	versionPrefix := strings.Repeat("../", depth)
	rootPrefix := versionPrefix + "../"
	cssHref := versionPrefix + "assets/docs.css"

	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html lang=\"en\">\n<head>\n")
	b.WriteString("<meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	fmt.Fprintf(&b, "<title>%s · VendKit docs %s</title>\n",
		html.EscapeString(p.title), html.EscapeString(p.version))
	fmt.Fprintf(&b, "<link rel=\"stylesheet\" href=\"%s\">\n", html.EscapeString(cssHref))
	b.WriteString(themeBootScript)
	b.WriteString("</head>\n<body>\n")

	// Top bar: brand + version selector + theme toggle.
	b.WriteString("<header class=\"vk-top\"><div class=\"vk-topwrap\">\n")
	fmt.Fprintf(&b, "<a class=\"vk-brand\" href=\"%sindex.html\"><span class=\"vk-bdot\"></span>vendkit docs</a>\n", versionPrefix)
	b.WriteString(versionSelector(p, rootPrefix))
	b.WriteString(themeToggle)
	b.WriteString("</div></header>\n")

	b.WriteString("<div class=\"vk-shell\">\n")
	b.WriteString(renderNav(p, versionPrefix))
	b.WriteString("<main class=\"vk-main\"><article class=\"vk-doc\">\n")
	b.WriteString(p.bodyHTML)
	b.WriteString("\n</article>\n")
	fmt.Fprintf(&b, "<footer class=\"vk-foot\">VendKit documentation · <code>%s</code> · "+
		"<a href=\"%s\">all releases</a></footer>\n",
		html.EscapeString(p.version), html.EscapeString(rootPrefix))
	b.WriteString("</main>\n</div>\n")

	fmt.Fprintf(&b, "%s\n", versionSelectorScript)
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

// renderNav builds the left navigation, grouping docs by section and marking
// the current page active.
func renderNav(p pageData, versionPrefix string) string {
	var b strings.Builder
	b.WriteString("<nav class=\"vk-nav\" aria-label=\"Documentation\">\n")
	fmt.Fprintf(&b, "<a class=\"vk-nav-home%s\" href=\"%sindex.html\">Overview</a>\n",
		activeClass(p.rel == "index.html"), versionPrefix)
	lastSection := "\x00"
	for _, d := range p.docs {
		if d.section != lastSection {
			if lastSection != "\x00" {
				b.WriteString("</ul>\n")
			}
			if d.section != "" {
				fmt.Fprintf(&b, "<div class=\"vk-nav-sec\">%s</div>\n", html.EscapeString(titleCase(d.section)))
			} else {
				b.WriteString("<div class=\"vk-nav-sec\">Guides</div>\n")
			}
			b.WriteString("<ul class=\"vk-nav-list\">\n")
			lastSection = d.section
		}
		fmt.Fprintf(&b, "  <li><a class=\"vk-nav-link%s\" href=\"%s%s\">%s</a></li>\n",
			activeClass(d.rel == p.rel),
			versionPrefix, html.EscapeString(d.rel), html.EscapeString(d.title))
	}
	if lastSection != "\x00" {
		b.WriteString("</ul>\n")
	}
	b.WriteString("</nav>\n")
	return b.String()
}

func activeClass(active bool) string {
	if active {
		return " is-active"
	}
	return ""
}

// versionSelector renders the dropdown. Options are baked from the current
// manifest snapshot (so it works without JS and off the file:// protocol);
// versionSelectorScript re-fetches versions.json at runtime so older pages
// surface newer versions on the live site.
func versionSelector(p pageData, rootPrefix string) string {
	var b strings.Builder
	fmt.Fprintf(&b,
		"<select class=\"vk-vers\" aria-label=\"Documentation version\" "+
			"data-current=\"%s\" data-root=\"%s\" data-rel=\"%s\">\n",
		html.EscapeString(p.version), html.EscapeString(rootPrefix), html.EscapeString(p.rel))
	for _, v := range p.manifest.Versions {
		label := v.Version
		if v.Latest {
			label += " (latest)"
		}
		sel := ""
		if v.Version == p.version {
			sel = " selected"
		}
		fmt.Fprintf(&b, "  <option value=\"%s\"%s>%s</option>\n",
			html.EscapeString(v.Version), sel, html.EscapeString(label))
	}
	b.WriteString("</select>\n")
	return b.String()
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// themeBootScript applies a persisted light/dark preference before paint,
// mirroring the landing page's approach.
const themeBootScript = `<script>(function(){try{var t=localStorage.getItem('vk-theme');if(t==='dark'||t==='light')document.documentElement.setAttribute('data-theme',t);}catch(e){}})();</script>
`

const themeToggle = `<button class="vk-tbtn" type="button" aria-label="Toggle color scheme" onclick="(function(d){var c=d.getAttribute('data-theme');if(!c){c=(window.matchMedia&&matchMedia('(prefers-color-scheme: dark)').matches)?'dark':'light';}var n=c==='dark'?'light':'dark';d.setAttribute('data-theme',n);try{localStorage.setItem('vk-theme',n);}catch(e){}})(document.documentElement)">◑</button>
`

// versionSelectorScript wires the dropdown: navigate on change, and (best
// effort) refresh the option list from versions.json so a page published for an
// older release still lists releases added later. Navigation works from the
// baked options even if the fetch fails (e.g. file://).
const versionSelectorScript = `<script>
(function(){
  var sel=document.querySelector('select.vk-vers');
  if(!sel)return;
  var root=sel.getAttribute('data-root')||'';
  var rel=sel.getAttribute('data-rel')||'index.html';
  var current=sel.getAttribute('data-current');
  sel.addEventListener('change',function(){
    if(sel.value)location.href=root+sel.value+'/'+rel;
  });
  try{
    fetch(root+'versions.json',{cache:'no-cache'}).then(function(r){return r.json();}).then(function(m){
      if(!m||!m.versions||!m.versions.length)return;
      sel.innerHTML='';
      m.versions.forEach(function(v){
        var o=document.createElement('option');
        o.value=v.version;
        o.textContent=v.version+(v.latest?' (latest)':'');
        if(v.version===current)o.selected=true;
        sel.appendChild(o);
      });
    }).catch(function(){});
  }catch(e){}
})();
</script>`
