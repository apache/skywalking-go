package profile

import (
	"context"
	"runtime/pprof"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLabels(t *testing.T) {
	var ctx = context.Background()
	labels := pprof.Labels("test1", "test1_label", "test2", "test2_label")
	ctx = pprof.WithLabels(ctx, labels)
	pprof.SetGoroutineLabels(ctx)
	ls := GetPprofLabelSet()
	ts := LabelSet{list: []label{{"test1", "test1_label"}, {"test2", "test2_label"}}}
	assert.Equal(t, ts, *ls)
}

func TestSetLabels(t *testing.T) {
	ts := &LabelSet{list: []label{{"test1", "test1_label"}, {"test2", "test2_label"}}}
	labels := Labels(ts, "test3", "test3_label")
	SetGoroutineLabels(labels)
	re := GetPprofLabelSet()
	assert.Equal(t, re, ts)
}
