package requestid

import "context"

const Header = "X-Request-ID"

type key struct{}

func Valid(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, c := range []byte(id) {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == ':' || c == '-' {
			continue
		}
		return false
	}
	return true
}

func With(ctx context.Context, id string) context.Context {
	if !Valid(id) {
		id = ""
	}
	return context.WithValue(ctx, key{}, id)
}

func Get(ctx context.Context) string {
	id, _ := ctx.Value(key{}).(string)
	return id
}
