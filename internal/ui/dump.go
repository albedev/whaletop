package ui

import (
	"context"
	"time"

	"github.com/albedev/dtop/internal/collector"
	"github.com/albedev/dtop/internal/docker"
)

// Dump renders single frames of both views without a TTY (dtop --dump 160x45).
// It drives the model with the same messages the program loop would deliver,
// so it doubles as a smoke test of the whole pipeline.
func Dump(ctx context.Context, cli *docker.Client, col *collector.Collector, opt Options, w, h int) []string {
	m := New(ctx, cli, col, opt)
	m.w, m.h = w, h
	m.sampling, m.diskLoading = false, false
	// two samples: CPU% and rates need a previous sample to diff against
	for i := 0; i < 2; i++ {
		mm, _ := m.Update(snapMsg(col.Sample(ctx)))
		m = mm.(Model)
		if i == 0 {
			time.Sleep(opt.Interval)
		}
	}
	mm, _ := m.Update(diskMsg(col.Disk(ctx, opt.DiskLimit)))
	m = mm.(Model)
	res := m.View()
	m.view = viewDisk
	m.rebuildDisk()
	return []string{res, m.View()}
}
