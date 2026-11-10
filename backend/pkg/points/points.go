// Package points represents exact hundredths of an authored problem point.
package points

import (
	"bytes"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"gopkg.in/yaml.v3"
)

type Value int64

const Unit Value = 100
const Max Value = 1000000000 * Unit

func Parse(text string) (Value, error) {
	if text == "" || strings.TrimSpace(text) != text || strings.HasPrefix(text, "-") || strings.ContainsAny(text, "eE+") {
		return 0, fmt.Errorf("points must be a nonnegative decimal with up to two fractional digits")
	}
	whole, fraction, has := strings.Cut(text, ".")
	if whole == "" || (has && (len(fraction) == 0 || len(fraction) > 2)) {
		return 0, fmt.Errorf("invalid point precision")
	}
	for _, c := range whole + fraction {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid decimal points")
		}
	}
	n, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || n > int64(Max/Unit) {
		return 0, fmt.Errorf("points exceed limit")
	}
	for len(fraction) < 2 {
		fraction += "0"
	}
	f, _ := strconv.ParseInt(fraction, 10, 64)
	value := Value(n)*Unit + Value(f)
	if value > Max {
		return 0, fmt.Errorf("points exceed limit")
	}
	return value, nil
}
func (v Value) String() string   { return fmt.Sprintf("%d.%02d", int64(v)/100, int64(v)%100) }
func (v Value) Float64() float64 { return float64(v) / 100 }
func (v Value) MarshalJSON() ([]byte, error) {
	if v < 0 || v > Max {
		return nil, fmt.Errorf("invalid points")
	}
	return []byte(v.String()), nil
}
func (v *Value) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return fmt.Errorf("points cannot be null")
	}
	parsed, err := Parse(string(data))
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}
func (v Value) MarshalYAML() (any, error) {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: v.String()}, nil
}
func (v *Value) UnmarshalYAML(node *yaml.Node) error {
	parsed, err := Parse(node.Value)
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}
func (v Value) NumericValue() (pgtype.Numeric, error) {
	if v < 0 || v > Max {
		return pgtype.Numeric{}, fmt.Errorf("invalid points")
	}
	return pgtype.Numeric{Int: big.NewInt(int64(v)), Exp: -2, Valid: true}, nil
}
func (v *Value) ScanNumeric(n pgtype.Numeric) error {
	if !n.Valid || n.NaN || n.InfinityModifier != pgtype.Finite || n.Int == nil {
		return fmt.Errorf("invalid numeric points")
	}
	cents := new(big.Int).Set(n.Int)
	exp := int64(n.Exp) + 2
	if exp > 20 || exp < -20 {
		return fmt.Errorf("invalid numeric point scale")
	}
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(max(exp, -exp)), nil)
	if exp >= 0 {
		cents.Mul(cents, factor)
	} else {
		q, r := new(big.Int), new(big.Int)
		q.QuoRem(cents, factor, r)
		if r.Sign() != 0 {
			return fmt.Errorf("numeric points exceed precision")
		}
		cents = q
	}
	if !cents.IsInt64() || cents.Sign() < 0 || cents.Int64() > int64(Max) {
		return fmt.Errorf("numeric points exceed limit")
	}
	*v = Value(cents.Int64())
	return nil
}
