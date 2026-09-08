package mirror

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var htmlAttrRe = regexp.MustCompile(`(?is)(?:^|[\t\n\f\r ])(href|src)[\t\n\f\r ]*=[\t\n\f\r ]*("([^"]*)"|'([^']*)'|([^\t\n\f\r >]+))`)

func (m *Mirror) processHTML(content string, base *url.URL, currentLocalPath string) (string, []*url.URL) {
	var result strings.Builder
	var links []*url.URL
	lower := strings.ToLower(content)

	for pos := 0; pos < len(content); {
		relativeStart := strings.IndexByte(content[pos:], '<')
		if relativeStart < 0 {
			result.WriteString(content[pos:])
			break
		}

		start := pos + relativeStart
		result.WriteString(content[pos:start])

		if strings.HasPrefix(content[start:], "<!--") {
			end := strings.Index(content[start+4:], "-->")
			if end < 0 {
				result.WriteString(content[start:])
				break
			}
			end += start + 7
			result.WriteString(content[start:end])
			pos = end
			continue
		}

		end := findTagEnd(content, start)
		if end < 0 {
			result.WriteString(content[start:])
			break
		}

		tag := content[start:end]
		name, closing := tagName(tag)
		if closing || name == "" {
			result.WriteString(tag)
			pos = end
			continue
		}

		newTag, found := m.processHTMLTag(tag, name, base, currentLocalPath)
		result.WriteString(newTag)
		links = append(links, found...)
		pos = end

		if name != "script" && name != "style" {
			continue
		}

		closeStart := findClosingTag(lower, pos, name)
		if closeStart < 0 {
			result.WriteString(content[pos:])
			break
		}

		body := content[pos:closeStart]
		if name == "style" {
			var styleLinks []*url.URL
			body, styleLinks = m.rewriteCSSURLs(body, base, currentLocalPath)
			links = append(links, styleLinks...)
		}
		result.WriteString(body)

		closeEnd := findTagEnd(content, closeStart)
		if closeEnd < 0 {
			result.WriteString(content[closeStart:])
			break
		}
		result.WriteString(content[closeStart:closeEnd])
		pos = closeEnd
	}

	return result.String(), links
}

func (m *Mirror) processHTMLTag(tag, name string, base *url.URL, currentLocalPath string) (string, []*url.URL) {
	attribute := linkAttribute(name)
	if attribute == "" {
		return tag, nil
	}

	for _, match := range htmlAttrRe.FindAllStringSubmatchIndex(tag, -1) {
		if !strings.EqualFold(tag[match[2]:match[3]], attribute) {
			continue
		}

		quotedStart, quotedEnd := match[4], match[5]
		value := submatchValue(tag, match, 6, 8, 10)
		target, err := resolveLink(base, value)
		if err != nil || !m.shouldMirror(target) {
			return tag, nil
		}

		if !m.shouldRewriteLink(value) {
			return tag, []*url.URL{target}
		}

		quoted := tag[quotedStart:quotedEnd]
		rel := relativeLink(currentLocalPath, m.localPath(target))
		converted := tag[:quotedStart] + quoteLike(quoted, rel) + tag[quotedEnd:]
		return converted, []*url.URL{target}
	}

	return tag, nil
}

func (m *Mirror) shouldRewriteLink(value string) bool {
	if m.opts.ConvertLinks {
		return true
	}

	ref, err := url.Parse(strings.TrimSpace(value))
	if err != nil || ref.IsAbs() || ref.Host != "" {
		return false
	}
	return strings.HasPrefix(ref.Path, "/") || strings.HasSuffix(ref.Path, "/") ||
		ref.Path != "" && filepath.Ext(ref.Path) == ""
}

func linkAttribute(tag string) string {
	switch tag {
	case "a", "link":
		return "href"
	case "audio", "iframe", "img", "script", "source", "video":
		return "src"
	default:
		return ""
	}
}

func submatchValue(s string, match []int, starts ...int) string {
	for _, startIndex := range starts {
		if match[startIndex] >= 0 {
			return s[match[startIndex]:match[startIndex+1]]
		}
	}
	return ""
}

func findTagEnd(content string, start int) int {
	var quote byte
	for i := start + 1; i < len(content); i++ {
		switch {
		case quote != 0 && content[i] == quote:
			quote = 0
		case quote == 0 && (content[i] == '\'' || content[i] == '"'):
			quote = content[i]
		case quote == 0 && content[i] == '>':
			return i + 1
		}
	}
	return -1
}

func tagName(tag string) (string, bool) {
	i := 1
	for i < len(tag) && isHTMLSpace(tag[i]) {
		i++
	}

	closing := i < len(tag) && tag[i] == '/'
	if closing {
		i++
		for i < len(tag) && isHTMLSpace(tag[i]) {
			i++
		}
	}

	start := i
	for i < len(tag) && isTagNameByte(tag[i]) {
		i++
	}
	if start == i {
		return "", closing
	}
	return strings.ToLower(tag[start:i]), closing
}

func findClosingTag(lower string, start int, name string) int {
	needle := "</" + name
	for start < len(lower) {
		relative := strings.Index(lower[start:], needle)
		if relative < 0 {
			return -1
		}

		match := start + relative
		after := match + len(needle)
		if after >= len(lower) || lower[after] == '>' || isHTMLSpace(lower[after]) {
			return match
		}
		start = after
	}
	return -1
}

func isHTMLSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\f' || b == '\r'
}

func isTagNameByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' ||
		b >= '0' && b <= '9' || b == '-' || b == ':'
}
