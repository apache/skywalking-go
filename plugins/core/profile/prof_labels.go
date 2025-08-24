package profile

import (
	"slices"
	"strings"
	"unsafe"
)

// 复制 runtime/pprof 内部类型

type label struct {
	key   string
	value string
}

type LabelSet struct {
	list []label
}

type labelMap struct {
	LabelSet
}

//go:linkname runtimeGetProfLabel runtime/pprof.runtime_getProfLabel
func runtimeGetProfLabel() unsafe.Pointer

//go:linkname runtimeSetProfLabel runtime/pprof.runtime_setProfLabel
func runtimeSetProfLabel(label unsafe.Pointer)

func GetPprofLabelSet() *LabelSet {
	ptr := runtimeGetProfLabel()
	if ptr != nil {
		lm := (*labelMap)(ptr)
		if lm != nil && lm.list != nil {
			return &lm.LabelSet
		} else {
			return &LabelSet{list: make([]label, 0)}
		}
	} else {
		return &LabelSet{list: make([]label, 0)}
	}

}

func Labels(s *LabelSet, args ...string) *LabelSet {
	if len(args)%2 != 0 {
		panic("uneven number of arguments to pprof.Labels")
	}

	// 先追加
	for i := 0; i < len(args); i += 2 {
		s.list = append(s.list, label{key: args[i], value: args[i+1]})
	}

	// 排序
	slices.SortStableFunc(s.list, func(a, b label) int {
		return strings.Compare(a.key, b.key)
	})

	// 去重：如果 key 相同，用最新的覆盖
	deduped := make([]label, 0, len(s.list))
	for i, lbl := range s.list {
		if i == 0 || lbl.key != s.list[i-1].key {
			deduped = append(deduped, lbl)
		} else {
			deduped[len(deduped)-1] = lbl
		}
	}
	s.list = deduped

	return s
}

func SetGoroutineLabels(s *LabelSet) {
	runtimeSetProfLabel(unsafe.Pointer(s))
}
