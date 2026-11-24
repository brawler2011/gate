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

// SplitHTML cuts a Polygon HTML statement into section fragments that pandoc reads with their math;
// a page without Polygon's sections becomes the legend.
func SplitHTML(page string) (map[string]string, error) {
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
	for name, fragment := range result {
		result[name] = polygonMath.ReplaceAllStringFunc(fragment, func(m string) string {
			parts := polygonMath.FindStringSubmatch(m)
			if parts[1] != "" {
				return `<span class="math display">\[` + parts[1] + `\]</span>`
			}
			return `<span class="math inline">\(` + parts[2] + `\)</span>`
		})
	}
	return result, nil
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
