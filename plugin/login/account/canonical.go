package account

import (
	"errors"
	"github.com/MoeForge/moego/ojson"
	"sort"
	"strconv"
	"strings"
)

func canonicalJSON(raw []byte) (string, error) {
	if !ojson.ValidBytes(raw) {
		return "", errors.New("identity_unverifiable")
	}
	var encode func(ojson.Result) (string, error)
	encode = func(value ojson.Result) (string, error) {
		if value.IsObject() {
			fields := map[string]ojson.Result{}
			keys := []string{}
			duplicate := false
			value.ForEach(func(key, child ojson.Result) bool {
				name := key.String()
				if _, found := fields[name]; found {
					duplicate = true
					return false
				}
				keys = append(keys, name)
				fields[name] = child
				return true
			})
			if duplicate {
				return "", errors.New("identity_unverifiable")
			}
			sort.Strings(keys)
			parts := make([]string, 0, len(keys))
			for _, key := range keys {
				child, err := encode(fields[key])
				if err != nil {
					return "", err
				}
				parts = append(parts, strconv.Quote(key)+":"+child)
			}
			return "{" + strings.Join(parts, ",") + "}", nil
		}
		if value.IsArray() {
			parts := []string{}
			for _, child := range value.Array() {
				encoded, err := encode(child)
				if err != nil {
					return "", err
				}
				parts = append(parts, encoded)
			}
			return "[" + strings.Join(parts, ",") + "]", nil
		}
		if value.Type == ojson.String {
			return strconv.Quote(value.String()), nil
		}
		return value.Raw, nil
	}
	return encode(ojson.ParseBytes(raw))
}
