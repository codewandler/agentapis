package sortmap

import (
	"encoding/json"
	"sort"
)

type SortedMap struct {
	keys   []string
	values []any
}

func NewSortedMap(m map[string]any) *SortedMap {
	sm := &SortedMap{
		keys:   make([]string, 0, len(m)),
		values: make([]any, 0, len(m)),
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		sm.keys = append(sm.keys, k)
		sm.values = append(sm.values, sortedMapValue(m[k]))
	}
	return sm
}

func sortedMapValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return NewSortedMap(val)
	case []any:
		out := make([]any, len(val))
		for i, elem := range val {
			out[i] = sortedMapValue(elem)
		}
		return out
	default:
		return v
	}
}

func (sm *SortedMap) MarshalJSON() ([]byte, error) {
	buf := make([]byte, 0, 2+len(sm.keys)*16)
	buf = append(buf, '{')

	for i, k := range sm.keys {
		if i > 0 {
			buf = append(buf, ',')
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf = append(buf, kb...)
		buf = append(buf, ':')

		vb, err := json.Marshal(sm.values[i])
		if err != nil {
			return nil, err
		}
		buf = append(buf, vb...)
	}

	buf = append(buf, '}')
	return buf, nil
}
