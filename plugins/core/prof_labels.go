package core

import (
	"context"
	"runtime/pprof"
	"slices"
	"strings"
	"unsafe"
)

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

func (m *ProfileManager) GetPprofLabelSet() interface{} {
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

func (m *ProfileManager) TurnToPprofLabel(l interface{}) interface{} {
	li := l.(*LabelSet).List()
	re := pprof.Labels(li...)
	return re
}

func GetLabelsFromCtx(ctx context.Context) LabelSet {
	var labels LabelSet
	// 使用公共 API ForLabels 迭代上下文标签
	pprof.ForLabels(ctx, func(key, value string) bool {
		labels.list = append(labels.list, label{key: key, value: value})
		return true // 继续迭代所有标签
	})
	return labels
}

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
		panic("uneven number of arguments to profile.Labels")
	}

	// add first
	for i := 0; i < len(args); i += 2 {
		s.list = append(s.list, label{key: args[i], value: args[i+1]})
	}

	// sort
	slices.SortStableFunc(s.list, func(a, b label) int {
		return strings.Compare(a.key, b.key)
	})

	// remove duplicates
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

func (s *LabelSet) List() []string {
	var ret []string
	for _, v := range s.list {
		ret = append(ret, v.key)
		ret = append(ret, v.value)
	}
	return ret
}

func SetGoroutineLabels(s *LabelSet) {
	runtimeSetProfLabel(unsafe.Pointer(s))
}
