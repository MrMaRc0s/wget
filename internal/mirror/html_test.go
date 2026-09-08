package mirror

import (
	"net/url"
	"reflect"
	"testing"
)

func TestProcessHTMLFindsResourcesOutsideScriptBodies(t *testing.T) {
	base := mustParseURL(t, "https://example.com/")
	m := New(Options{RootURL: base.String()})
	m.baseHost = base.Host

	content := `<html>
<head>
  <link href="/site.css" rel="stylesheet">
  <script src="/app.js"></script>
  <script>
    card.innerHTML = '<img src="' + item.image + '">';
  </script>
</head>
<body>
  <a href="/about">About</a>
  <img src="/image.png">
  <video src="/movie.mp4"></video>
  <audio src="/sound.mp3"></audio>
  <source src="/movie.webm">
  <iframe src="/frame"></iframe>
</body>
</html>`
	wantContent := `<html>
<head>
  <link href="site.css" rel="stylesheet">
  <script src="app.js"></script>
  <script>
    card.innerHTML = '<img src="' + item.image + '">';
  </script>
</head>
<body>
  <a href="about.html">About</a>
  <img src="image.png">
  <video src="movie.mp4"></video>
  <audio src="sound.mp3"></audio>
  <source src="movie.webm">
  <iframe src="frame.html"></iframe>
</body>
</html>`

	gotContent, gotLinks := m.processHTML(content, base, "index.html")
	wantLinks := []string{
		"https://example.com/site.css",
		"https://example.com/app.js",
		"https://example.com/about",
		"https://example.com/image.png",
		"https://example.com/movie.mp4",
		"https://example.com/sound.mp3",
		"https://example.com/movie.webm",
		"https://example.com/frame",
	}

	if gotContent != wantContent {
		t.Fatalf("content = %q, want %q", gotContent, wantContent)
	}
	if got := urlStrings(gotLinks); !reflect.DeepEqual(got, wantLinks) {
		t.Fatalf("links = %#v, want %#v", got, wantLinks)
	}
}

func TestProcessHTMLConvertsResourceTags(t *testing.T) {
	base := mustParseURL(t, "https://example.com/docs/page.html")
	m := New(Options{RootURL: base.String(), ConvertLinks: true})
	m.baseHost = base.Host

	content := `<script src="/js/app.js"></script><iframe src='/frame'></iframe>`
	want := `<script src="../js/app.js"></script><iframe src='../frame.html'></iframe>`

	got, links := m.processHTML(content, base, "docs/page.html")
	if got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	if len(links) != 2 {
		t.Fatalf("got %d links, want 2", len(links))
	}
}

func TestProcessHTMLRewritesPathsThatDoNotWorkFromDisk(t *testing.T) {
	base := mustParseURL(t, "https://example.com/docs/page.html")
	m := New(Options{RootURL: base.String()})
	m.baseHost = base.Host

	content := `<link href="/css/site.css"><script src="scripts/app.js"></script><a href="about">About</a><a href="https://example.com/contact">Contact</a>`
	want := `<link href="../css/site.css"><script src="scripts/app.js"></script><a href="about.html">About</a><a href="https://example.com/contact">Contact</a>`

	got, links := m.processHTML(content, base, "docs/page.html")
	if got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	if len(links) != 4 {
		t.Fatalf("got %d links, want 4", len(links))
	}
}

func TestProcessHTMLFindsInlineStyleResources(t *testing.T) {
	base := mustParseURL(t, "https://example.com/")
	m := New(Options{RootURL: base.String(), ConvertLinks: true})
	m.baseHost = base.Host

	content := `<style>.hero { background: url('/hero.png'); }</style>`
	want := `<style>.hero { background: url(hero.png); }</style>`

	got, links := m.processHTML(content, base, "index.html")
	if got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	if got := urlStrings(links); !reflect.DeepEqual(got, []string{"https://example.com/hero.png"}) {
		t.Fatalf("links = %#v", got)
	}
}

func TestProcessCSSRewritesRootRelativePaths(t *testing.T) {
	base := mustParseURL(t, "https://example.com/css/site.css")
	m := New(Options{RootURL: base.String()})
	m.baseHost = base.Host

	content := `.hero { background: url('/images/hero.png'); }`
	want := `.hero { background: url(../images/hero.png); }`

	got, links := m.processCSS(content, base, "css/site.css")
	if got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
	if len(links) != 1 {
		t.Fatalf("got %d links, want 1", len(links))
	}
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func urlStrings(urls []*url.URL) []string {
	result := make([]string, len(urls))
	for i, u := range urls {
		result[i] = u.String()
	}
	return result
}
