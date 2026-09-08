package mirror

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

// Matches url(...) references inside CSS, e.g. background images, fonts,
// and @import. Go's RE2 engine has no backreferences, so quoted and
// unquoted forms are spelled out as separate alternatives.
var cssURLRe = regexp.MustCompile(`(?i)url\(\s*(?:"([^"]*)"|'([^']*)'|([^'")\s][^'")]*))\s*\)`)

// processCSS mirrors url(...) references found in a standalone
// stylesheet, so it keeps its fonts and background images.
func (m *Mirror) processCSS(content string, base *url.URL, currentLocalPath string) (string, []*url.URL) {
	return m.rewriteCSSURLs(content, base, currentLocalPath)
}

// rewriteCSSURLs is the shared engine behind processCSS and inline
// <style> handling: find url(...) references, resolve them, and
// optionally rewrite the mirrored ones to a local relative path.
func (m *Mirror) rewriteCSSURLs(css string, base *url.URL, currentLocalPath string) (string, []*url.URL) {
	var links []*url.URL

	result := cssURLRe.ReplaceAllStringFunc(css, func(match string) string {
		sub := cssURLRe.FindStringSubmatch(match)
		value := firstNonEmpty(sub[1], sub[2], sub[3])

		target, err := resolveLink(base, value)
		if err != nil || !m.shouldMirror(target) {
			return match
		}
		links = append(links, target)

		if !m.opts.ConvertLinks {
			return match
		}
		rel := relativeLink(currentLocalPath, m.localPath(target))
		return "url(" + rel + ")"
	})

	return result, links
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func resolveLink(base *url.URL, value string) (*url.URL, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errSkip
	}
	for _, prefix := range []string{"#", "mailto:", "javascript:", "data:", "tel:"} {
		if strings.HasPrefix(value, prefix) {
			return nil, errSkip
		}
	}

	ref, err := url.Parse(value)
	if err != nil {
		return nil, err
	}
	resolved := base.ResolveReference(ref)
	resolved.Fragment = ""
	return resolved, nil
}

// relativeLink computes the path used from fromLocalPath's directory to
// reach toLocalPath, both relative to the mirror root.
func relativeLink(fromLocalPath, toLocalPath string) string {
	rel, err := filepath.Rel(filepath.Dir(fromLocalPath), toLocalPath)
	if err != nil {
		return toLocalPath
	}
	return filepath.ToSlash(rel)
}

// quoteLike re-wraps a replacement value using the same quoting (or lack
// thereof) as the original attribute value it replaces.
func quoteLike(original, replacement string) string {
	switch {
	case strings.HasPrefix(original, `"`):
		return `"` + replacement + `"`
	case strings.HasPrefix(original, `'`):
		return `'` + replacement + `'`
	default:
		return replacement
	}
}
