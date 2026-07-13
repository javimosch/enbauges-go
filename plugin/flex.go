package plugin

import (
	"encoding/json"
	"strconv"
	"strings"
)

// FlexInt decodes from a JSON number, a numeric string ("2" — what Vue
// v-model on <select>/<input> often sends), or null. The Node app coerced
// everything through Number(); Go plugins use this to stay as forgiving
// instead of failing the whole body decode.
type FlexInt int

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == `""` {
		*f = 0
		return nil
	}
	s = strings.Trim(s, `"`)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*f = 0
		return nil // Number(garbage) is NaN → Node treated it as 0 downstream
	}
	*f = FlexInt(int(v))
	return nil
}

func (f FlexInt) MarshalJSON() ([]byte, error) { return json.Marshal(int(f)) }

// FlexInts converts a slice of FlexInt to plain ints.
func FlexInts(in []FlexInt) []int {
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}
