package installedwebapp

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

func parseDesktop(data []byte) (map[string]string, bool) {
	fields := make(map[string]string)
	group := ""
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSuffix(raw, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			group = line[1 : len(line)-1]
			continue
		}
		if group != "Desktop Entry" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || !validDesktopKey(key) {
			return nil, false
		}
		fields[key] = desktopUnescape(value)
	}
	return fields, group == "Desktop Entry"
}

type execParser struct {
	argv    []string
	token   strings.Builder
	quoted  bool
	escaped bool
}

func parseExec(line string) ([]string, int, string, string, bool) {
	parser := execParser{}
	for _, r := range line {
		if !parser.consume(r) {
			return nil, -1, "", "", false
		}
	}
	if parser.escaped || parser.quoted {
		return nil, -1, "", "", false
	}
	parser.flush()
	if len(parser.argv) == 0 {
		return nil, -1, "", "", false
	}
	return execURL(parser.argv)
}

func (p *execParser) consume(r rune) bool {
	if p.escaped {
		return p.consumeEscaped(r)
	}
	if r == '\\' {
		p.escaped = true
		return true
	}
	if r == '"' {
		p.quoted = !p.quoted
		return true
	}
	if unicode.IsSpace(r) && !p.quoted {
		p.flush()
		return true
	}
	if execRejectedRune(r, p.quoted) {
		return false
	}
	p.token.WriteRune(r)
	return true
}

func execRejectedRune(r rune, quoted bool) bool {
	return r == '%' || unicode.IsControl(r) || (r == '\'' && !quoted)
}

func (p *execParser) consumeEscaped(r rune) bool {
	if r == '%' || unicode.IsControl(r) {
		return false
	}
	p.token.WriteRune(r)
	p.escaped = false
	return true
}

func (p *execParser) flush() {
	if p.token.Len() > 0 {
		p.argv = append(p.argv, p.token.String())
		p.token.Reset()
	}
}

func execURL(argv []string) ([]string, int, string, string, bool) {
	urlIndex, urlFlag, execURL := -1, "", ""
	for i, arg := range argv[1:] {
		index := i + 1
		candidate, flag := execURLCandidate(arg)
		if candidate == "" {
			continue
		}
		if !isHTTPURL(candidate) || urlIndex >= 0 {
			return nil, -1, "", "", false
		}
		urlIndex, urlFlag, execURL = index, flag, candidate
	}
	return argv, urlIndex, urlFlag, execURL, true
}

func execURLCandidate(arg string) (string, string) {
	if strings.HasPrefix(arg, "--app=") {
		return strings.TrimPrefix(arg, "--app="), "--app="
	}
	if isHTTPURL(arg) {
		return arg, ""
	}
	return "", ""
}

type plistState struct {
	values map[string]string
	key    string
	depth  int
	inDict bool
}

func parsePlistStrings(data []byte) (map[string]string, bool) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	state := plistState{values: make(map[string]string)}
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return state.values, state.inDict
		}
		if err != nil || !consumePlistToken(decoder, token, &state) {
			return nil, false
		}
	}
}

func consumePlistToken(decoder *xml.Decoder, token xml.Token, state *plistState) bool {
	element, isStart := token.(xml.StartElement)
	if isStart {
		return consumePlistStart(decoder, element, state)
	}
	end, isEnd := token.(xml.EndElement)
	return !isEnd || consumePlistEnd(end, state)
}

func consumePlistStart(decoder *xml.Decoder, element xml.StartElement, state *plistState) bool {
	switch element.Name.Local {
	case "dict":
		state.depth++
		state.inDict = true
		return state.depth <= 16
	case "key":
		var value string
		if err := decoder.DecodeElement(&value, &element); err != nil {
			return false
		}
		state.key = value
	case "string":
		var value string
		if err := decoder.DecodeElement(&value, &element); err != nil {
			return false
		}
		if state.key != "" {
			state.values[state.key] = value
			state.key = ""
		}
	}
	return true
}

func consumePlistEnd(element xml.EndElement, state *plistState) bool {
	if element.Name.Local == "dict" {
		state.depth--
	}
	return state.depth >= 0
}

func validateLaunchURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("installed web app cannot launch this URL")
	}
	return nil
}

func shortcutHasRootPath(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && (parsed.Path == "" || parsed.Path == "/") && parsed.RawPath == ""
}

func originOf(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("web app metadata has an invalid URL")
	}
	parsed.Path, parsed.RawPath, parsed.RawQuery, parsed.Fragment = "", "", "", ""
	return parsed.String(), nil
}

func knownBrowser(executable string) bool {
	base := strings.ToLower(filepath.Base(executable))
	switch base {
	case "brave", "brave-browser", "chrome", "chromium", "chromium-browser", "google-chrome", "google-chrome-stable":
		return true
	default:
		return false
	}
}

func knownMacBundle(bundleID string) bool {
	return strings.HasPrefix(bundleID, "com.google.Chrome.app.") || strings.HasPrefix(bundleID, "com.brave.Browser.app.")
}

func isHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.User == nil && parsed.Hostname() != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func validDesktopKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-') {
			return false
		}
	}
	return true
}

func desktopUnescape(value string) string {
	var out strings.Builder
	escaped := false
	for _, r := range value {
		if escaped {
			switch r {
			case 's':
				out.WriteByte(' ')
			case 'n':
				out.WriteByte('\n')
			case 't':
				out.WriteByte('\t')
			case 'r':
				out.WriteByte('\r')
			default:
				out.WriteRune(r)
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		out.WriteRune(r)
	}
	if escaped {
		out.WriteByte('\\')
	}
	return out.String()
}

func firstNonEmpty(fields map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(fields[key]); value != "" {
			return value
		}
	}
	return ""
}

func stableDiagnostics(values []string) []string {
	sort.Strings(values)
	return values
}

func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	var result []string
	for _, value := range values {
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
