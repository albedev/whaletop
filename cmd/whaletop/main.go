// whaletop is a btop-like terminal monitor for Docker: resources (CPU, memory,
// network, IO) relative to what docker may use, and disk usage of images,
// containers, volumes and build cache, with actions to stop/kill/restart/remove.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dustin/go-humanize"

	"github.com/albedev/whaletop/internal/collector"
	"github.com/albedev/whaletop/internal/docker"
	"github.com/albedev/whaletop/internal/ui"
)

var version = "dev" // overridden with -ldflags "-X main.version=..." (Makefile, GoReleaser)

// buildVersion falls back to the module version recorded by `go install …@vX.Y.Z`.
func buildVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}

func main() {
	var (
		host      = flag.String("host", "", "docker endpoint (default: DOCKER_HOST, current docker context, well-known sockets)")
		interval  = flag.Duration("interval", time.Second, "resources sampling interval")
		diskEvery = flag.Duration("disk-interval", 30*time.Second, "disk usage rescan interval while the disk view is open (x4 otherwise)")
		diskLimit = flag.String("disk-limit", "", "override detected docker disk capacity, e.g. 64G")
		graphMode = flag.String("graph", "block", "graph style: block or tty")
		all       = flag.Bool("all", true, "show stopped containers in the resources view")
		noMouse   = flag.Bool("no-mouse", false, "disable mouse support (keeps terminal text selection)")
		showVer   = flag.Bool("version", false, "print version and exit")
		dump      = flag.String("dump", "", "render one frame of each view at WxH to stdout and exit (debug)")
	)
	flag.Parse()
	if *showVer {
		fmt.Println("whaletop", buildVersion())
		return
	}

	opt := ui.Options{Interval: *interval, DiskInterval: *diskEvery, ShowStopped: *all}
	switch *graphMode {
	case "block":
		opt.Mode = ui.ModeBlock
	case "tty":
		opt.Mode = ui.ModeTTY
	default:
		fatal(fmt.Errorf("--graph must be block or tty, got %q", *graphMode))
	}
	if *diskLimit != "" {
		n, err := humanize.ParseBytes(*diskLimit)
		if err != nil {
			fatal(fmt.Errorf("--disk-limit: %w", err))
		}
		opt.DiskLimit = int64(n)
	}
	opt.Interval = max(opt.Interval, 250*time.Millisecond)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cli, err := docker.New(*host)
	if err != nil {
		fatal(err)
	}
	defer cli.Close()

	pctx, pcancel := context.WithTimeout(ctx, 5*time.Second)
	col, err := collector.New(pctx, cli)
	pcancel()
	if err != nil {
		fatal(fmt.Errorf("cannot reach docker at %s: %w\nis the daemon running? (Docker Desktop, colima, OrbStack...)", cli.Endpoint, err))
	}

	if *dump != "" {
		var w, h int
		if _, err := fmt.Sscanf(*dump, "%dx%d", &w, &h); err != nil {
			fatal(fmt.Errorf("--dump wants WxH, e.g. 160x45"))
		}
		for _, frame := range ui.Dump(ctx, cli, col, opt, w, h) {
			fmt.Println(frame)
		}
		return
	}

	if f := os.Getenv("WHALETOP_DEBUG"); f != "" {
		lf, err := tea.LogToFile(f, "whaletop")
		if err != nil {
			fatal(err)
		}
		defer lf.Close()
	}

	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithContext(ctx)}
	if !*noMouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	if _, err := tea.NewProgram(ui.New(ctx, cli, col, opt), opts...).Run(); err != nil && ctx.Err() == nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "whaletop:", err)
	os.Exit(1)
}
