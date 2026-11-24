package statement

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// polygonClasses map the divs of Polygon's HTML statements to sections; the header and samples are left out,
// since limits and samples come from the problem itself.
var polygonClasses = map[string]string{
	"legend": "legend", "input-specification": "input", "output-specification": "output",
	"interaction": "interaction", "scoring": "scoring", "note": "notes",
}

// polygonMath is MathJax as Polygon configures it: $$$ inline, $$$$$$ display.
var polygonMath = regexp.MustCompile(`(?s)\$\$\$\$\$\$(.+?)\$\$\$\$\$\$|\$\$\$(.+?)\$\$\$`)

// HTMLSection is a fragment of an HTML statement with its formulas held out as tokens,
// since pandoc's HTML reader would escape them as text.
type HTMLSection struct {
	HTML string
	Math []string
}

// LaTeX puts the formulas back into the fragment as pandoc converted it.
func (s HTMLSection) LaTeX(tex string) string {
	pairs := make([]string, 0, 2*len(s.Math))
	for i, m := range s.Math {
		pairs = append(pairs, mathToken(i), m)
	}
	return strings.NewReplacer(pairs...).Replace(tex)
}

func mathToken(i int) string { return fmt.Sprintf("GATEMATH%dEND", i) }

// SplitHTML cuts a Polygon HTML statement into section fragments; a page without Polygon's sections becomes the legend.
func SplitHTML(page string) (map[string]HTMLSection, error) {
	root, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return nil, fmt.Errorf("parse HTML statement: %w", err)
	}
	result := map[string]string{}
	var body *html.Node
	var walk func(*html.Node) error
	walk = func(n *html.Node) error {
		if n.DataAtom == atom.Body {
			body = n
		}
		if section, ok := polygonSection(n); ok {
			var buf bytes.Buffer
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if !hasClass(c, "section-title") {
					if err := html.Render(&buf, c); err != nil {
						return fmt.Errorf("render HTML: %w", err)
					}
				}
			}
			result[section] += buf.String()
			return nil
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := walk(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	if len(result) == 0 && body != nil {
		var buf bytes.Buffer
		for c := body.FirstChild; c != nil; c = c.NextSibling {
			if err := html.Render(&buf, c); err != nil {
				return nil, fmt.Errorf("render HTML: %w", err)
			}
		}
		result["legend"] = buf.String()
	}
	sections := make(map[string]HTMLSection, len(result))
	for name, fragment := range result {
		var s HTMLSection
		s.HTML = polygonMath.ReplaceAllStringFunc(fragment, func(m string) string {
			parts := polygonMath.FindStringSubmatch(m)
			formula := `\(` + html.UnescapeString(parts[2]) + `\)`
			if parts[1] != "" {
				formula = `\[` + html.UnescapeString(parts[1]) + `\]`
			}
			s.Math = append(s.Math, formula)
			return mathToken(len(s.Math) - 1)
		})
		sections[name] = s
	}
	return sections, nil
}

func polygonSection(n *html.Node) (string, bool) {
	if n.DataAtom != atom.Div {
		return "", false
	}
	for _, class := range classes(n) {
		if section, ok := polygonClasses[class]; ok {
			return section, true
		}
	}
	return "", false
}

func hasClass(n *html.Node, class string) bool { return slices.Contains(classes(n), class) }

func classes(n *html.Node) []string {
	for _, a := range n.Attr {
		if a.Key == "class" {
			return strings.Fields(a.Val)
		}
	}
	return nil
}
