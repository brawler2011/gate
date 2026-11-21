package spec

import (
	"bytes"
	"encoding/xml"
	"io"
	"unicode/utf8"

	"github.com/brawler2011/gate/backend/pkg"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"
)

// DecodeText returns data as UTF-8 and whether its bytes changed. Without a declared
// non-UTF-8 charset, text that is not UTF-8 is read as windows-1251, common in older Russian packages.
func DecodeText(data []byte, charset string) (string, bool, error) {
	decoder := charmap.Windows1251.NewDecoder()
	if charset != "" {
		enc, err := htmlindex.Get(charset)
		if err != nil {
			return "", false, Fail(pkg.ErrUnsupported, "", charset, "unsupported charset %s", charset)
		}
		if name, _ := htmlindex.Name(enc); name != "utf-8" {
			decoder = enc.NewDecoder()
		} else {
			charset = ""
		}
	}
	if charset == "" && utf8.Valid(data) {
		return string(data), false, nil
	}
	text, err := decoder.Bytes(data)
	if err != nil {
		return "", false, Fail(pkg.ErrPackageSyntax, "", charset, "decode %s: %v", charset, err)
	}
	return string(text), !bytes.Equal(text, data), nil
}

// UnmarshalXML accepts the encodings XML declarations name, such as windows-1251.
func UnmarshalXML(data []byte, v any) error {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		enc, err := htmlindex.Get(label)
		if err != nil {
			return nil, Fail(pkg.ErrUnsupported, "", label, "unsupported encoding %s", label)
		}
		return enc.NewDecoder().Reader(input), nil
	}
	if err := d.Decode(v); err != nil {
		return Fail(pkg.ErrPackageSyntax, "", "", "decode xml: %v", err)
	}
	return nil
}
