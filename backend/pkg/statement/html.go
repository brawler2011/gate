package statement

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/brawler2011/gate/backend/pkg/sandbox"
)

type Converter interface {
	HTML(ctx context.Context, tex string) (string, []sandbox.PandocMessage, error)
}

// Error is an author's mistake in a section; Line counts the section's lines, 0 when unknown.
type Error struct {
	Section string
	Line    int
	Message string
}

func (e *Error) Error() string {
	if e.Section == "" {
		return e.Message
	}
	if e.Line == 0 {
		return fmt.Sprintf("%s section: %s", e.Section, e.Message)
	}
	return fmt.Sprintf("%s section, line %d: %s", e.Section, e.Line, e.Message)
}

// Headings pandoc turns into ids mark where sections start; groups would scope the authors' macros.
const marker = "gate-section-"

// prelude gives pandoc the olymp.sty macros that the PDF has.
const prelude = "\\renewcommand{\\t}[1]{\\texttt{#1}}\n"

// HTML converts sections, given in Order, in one pandoc run. Content pandoc skips fails, so the site never misses what the PDF shows.
// Each image's src becomes the asset name graphicx would pick; an image with no asset fails.
func HTML(ctx context.Context, c Converter, sections []Section, exists func(name string) bool) (map[string]string, error) {
	var src strings.Builder
	src.WriteString(prelude)
	line := 1 + strings.Count(prelude, "\n")
	type span struct {
		name  string
		start int
	}
	var spans []span
	for _, s := range sections {
		if strings.TrimSpace(s.Text) == "" {
			continue
		}
		fmt.Fprintf(&src, "\\section*{%s%s}\n%s\n\n", marker, s.Name, s.Text)
		spans = append(spans, span{s.Name, line + 1})
		line += 3 + strings.Count(s.Text, "\n")
	}
	if len(spans) == 0 {
		return map[string]string{}, nil
	}
	out, log, err := c.HTML(ctx, src.String())
	if err != nil {
		return nil, err
	}
	for _, m := range log {
		if m.Type != "SkippedContent" {
			continue
		}
		e := &Error{Section: spans[0].name, Message: fmt.Sprintf("%s is not supported on the site", m.Contents)}
		for _, s := range spans {
			if m.Line >= s.start {
				e.Section, e.Line = s.name, m.Line-s.start+1
			}
		}
		return nil, e
	}
	result, err := split(out, exists)
	if err != nil {
		return nil, err
	}
	for _, s := range spans {
		if _, ok := result[s.name]; !ok {
			return nil, &Error{Section: s.name, Message: "the section is swallowed by the one before it; check its braces"}
		}
	}
	return result, nil
}

func split(out string, exists func(name string) bool) (map[string]string, error) {
	nodes, err := html.ParseFragment(strings.NewReader(out), &html.Node{Type: html.ElementNode, DataAtom: atom.Body, Data: "body"})
	if err != nil {
		return nil, fmt.Errorf("parse pandoc HTML: %w", err)
	}
	result := map[string]string{}
	var current string
	var buf bytes.Buffer
	flush := func() {
		if current != "" {
			result[current] = strings.TrimSpace(policy.Sanitize(buf.String()))
		}
		buf.Reset()
	}
	for _, n := range nodes {
		if name, ok := sectionName(n); ok {
			flush()
			current = name
			continue
		}
		if err := resolveImages(n, current, exists); err != nil {
			return nil, err
		}
		if err := html.Render(&buf, n); err != nil {
			return nil, fmt.Errorf("render HTML: %w", err)
		}
	}
	flush()
	return result, nil
}

func resolveImages(n *html.Node, section string, exists func(name string) bool) error {
	if n.DataAtom == atom.Img {
		for i, a := range n.Attr {
			if a.Key != "src" {
				continue
			}
			ref := a.Val
			if unescaped, err := url.PathUnescape(ref); err == nil {
				ref = unescaped
			}
			file, ok := ImageFile(ref, exists)
			if !ok {
				return &Error{Section: section, Message: fmt.Sprintf("image %q is not among the statement's assets", ref)}
			}
			n.Attr[i].Val = file
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if err := resolveImages(c, section, exists); err != nil {
			return err
		}
	}
	return nil
}

func sectionName(n *html.Node) (string, bool) {
	if n.DataAtom != atom.H1 {
		return "", false
	}
	for _, a := range n.Attr {
		if name, ok := strings.CutPrefix(a.Val, marker); a.Key == "id" && ok {
			return name, true
		}
	}
	return "", false
}

// The site shows images by asset name only and renders math with KaTeX.
var policy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "hr", "strong", "em", "b", "i", "u", "s", "del", "sub", "sup", "small", "code", "pre", "blockquote",
		"h1", "h2", "h3", "h4", "h5", "h6", "ul", "ol", "li", "dl", "dt", "dd", "table", "caption", "thead", "tbody", "tfoot", "tr", "th", "td",
		"figure", "figcaption", "span", "div")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^(math inline|math display|smallcaps|underline)$`)).OnElements("span")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^(center|flushleft|flushright)$`)).OnElements("div")
	p.AllowAttrs("start", "type").OnElements("ol")
	p.AllowAttrs("colspan", "rowspan").Matching(bluemonday.Integer).OnElements("td", "th")
	p.AllowStyles("text-align").MatchingEnum("left", "right", "center").OnElements("td", "th")
	p.AllowStyles("width", "height").Matching(regexp.MustCompile(`^[0-9.]+(cm|mm|in|pt|px|em|%)$`)).OnElements("img")
	p.AllowAttrs("alt").OnElements("img")
	p.AllowAttrs("src").Matching(regexp.MustCompile(`^[^:/?#]+$`)).OnElements("img")
	p.AllowAttrs("href").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.AllowRelativeURLs(true)
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return p
}()
