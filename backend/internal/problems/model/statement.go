package problemsmodel

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

var urlScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// ImageReferences resolves escapes and encoding as the renderer does; absolute and rooted URLs are excluded.
func ImageReferences(markdown string) []string {
	if !strings.Contains(markdown, "![") {
		return nil
	}
	source := []byte(markdown)
	doc := goldmark.DefaultParser().Parse(text.NewReader(source))
	var refs []string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if image, ok := n.(*ast.Image); ok && entering {
			destination := util.ResolveEntityNames(util.ResolveNumericReferences(util.UnescapePunctuations(image.Destination)))
			if name, ok := assetName(string(destination)); ok {
				refs = append(refs, name)
			}
		}
		return ast.WalkContinue, nil
	})
	return refs
}

func assetName(destination string) (string, bool) {
	if i := strings.IndexByte(destination, '#'); i >= 0 {
		destination = destination[:i]
	}
	if destination == "" || strings.HasPrefix(destination, "/") || urlScheme.MatchString(destination) {
		return "", false
	}
	destination = strings.TrimPrefix(destination, "./")
	if decoded, err := url.PathUnescape(destination); err == nil {
		destination = decoded
	}
	return destination, true
}
