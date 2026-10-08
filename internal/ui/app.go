// Package ui is the bubbletea front-end of dtop.
package ui

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/moby/moby/api/types/events"
	"github.com/moby/moby/client"

	"github.com/albedev/dtop/internal/collector"
	"github.com/albedev/dtop/internal/docker"
)

// Options are the command line settings.
type Options struct {
	Interval     time.Duration
	DiskInterval time.Duration
	DiskLimit    int64
	Mode         GraphMode
	ShowStopped  bool
}

type viewID int

const (
	viewResources viewID = iota
	viewDisk
)

type (
	tickMsg     struct{ gen int }
	diskTickMsg struct{} // one-shot (debounced event refresh)
	diskPollMsg struct{} // periodic check, decides whether a scan is due
	snapMsg     collector.Snapshot
	diskMsg     collector.DiskReport
	eventMsg    events.Message
	eventsErr   struct{ err error }
	actionMsg   struct {
		text string
		err  error
	}
)

type eventLine struct {
	at                  time.Time
	typ, action, target string
}

// Model is the root bubbletea model.
type Model struct {
	cli *docker.Client
	col *collector.Collector
	opt Options
	ctx context.Context

	w, h int
	view viewID

	snap     collector.Snapshot
	hasSnap  bool
	sampling bool
	tickGen  int // only the latest scheduled tick triggers a sample

	disk         collector.DiskReport
	hasDisk      bool
	diskLoading  bool
	diskQueued   bool // an event asked for a refresh while one was running
	diskDebounce bool

	res  resState
	dsk  diskState
	evts []eventLine
	evCh <-chan events.Message
	erCh <-chan error

	filter    textinput.Model
	filtering bool

	status    string
	statusErr bool
	statusAt  time.Time

	confirm *confirmState
	menu    *menuState
	logs    *logView
	help    bool
}

// New builds the model; call tea.NewProgram(m).Run().
func New(ctx context.Context, cli *docker.Client, col *collector.Collector, opt Options) Model {
	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = "filter"
	ti.CharLimit = 64
	m := Model{
		cli: cli, col: col, opt: opt, ctx: ctx,
		filter: ti,
		res:    resState{sort: sortCPU, desc: true, showStopped: opt.ShowStopped},
		dsk:    diskState{sort: dsortSize, desc: true},
	}
	m.subscribeEvents()
	// Init has a value receiver (bubbletea API): state it sets is lost, so the
	// in-flight flags for the first sample/scan are set here.
	m.sampling, m.diskLoading = true, true
	return m
}

func (m Model) Init() tea.Cmd {
	col, ctx, limit := m.col, m.ctx, m.opt.DiskLimit
	first := func() tea.Msg { return snapMsg(col.Sample(ctx)) }
	disk := func() tea.Msg { return diskMsg(col.Disk(ctx, limit)) }
	return tea.Batch(first, disk, m.waitEvent(), diskPoll())
}

// ---- commands ----

func (m *Model) sampleCmd() tea.Cmd {
	if m.sampling {
		return nil
	}
	m.sampling = true
	col, ctx := m.col, m.ctx
	return func() tea.Msg {
		c, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return snapMsg(col.Sample(c))
	}
}

func (m *Model) diskCmd() tea.Cmd {
	if m.diskLoading {
		m.diskQueued = true
		return nil
	}
	m.diskLoading = true
	col, ctx, limit := m.col, m.ctx, m.opt.DiskLimit
	return func() tea.Msg {
		c, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		return diskMsg(col.Disk(c, limit))
	}
}

func (m *Model) subscribeEvents() {
	r := m.cli.Events(m.ctx, client.EventsListOptions{})
	m.evCh, m.erCh = r.Messages, r.Err
}

func (m Model) waitEvent() tea.Cmd {
	ev, er := m.evCh, m.erCh
	return func() tea.Msg {
		select {
		case e, ok := <-ev:
			if !ok {
				return eventsErr{}
			}
			return eventMsg(e)
		case err := <-er:
			return eventsErr{err}
		}
	}
}

func tick(d time.Duration, gen int) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return tickMsg{gen} })
}

func diskPoll() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return diskPollMsg{} })
}

func (m *Model) run(f func(ctx context.Context) (string, error)) tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		c, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		text, err := f(c)
		return actionMsg{text, err}
	}
}

func (m *Model) setStatus(s string, isErr bool) {
	m.status, m.statusErr, m.statusAt = s, isErr, time.Now()
}

// ---- update ----

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	debugf("msg %T", msg)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		if m.logs != nil {
			m.logs.resize(m.w, m.h)
		}
		return m, nil

	case tickMsg:
		if msg.gen != m.tickGen {
			return m, nil
		}
		return m, m.sampleCmd()

	case snapMsg:
		m.sampling = false
		m.snap, m.hasSnap = collector.Snapshot(msg), true
		if msg.Err != nil {
			m.setStatus("docker: "+msg.Err.Error(), true)
		}
		m.rebuildRes()
		m.tickGen++
		return m, tick(m.opt.Interval, m.tickGen)

	case diskTickMsg:
		m.diskDebounce = false
		return m, m.diskCmd()

	case diskPollMsg:
		due := m.opt.DiskInterval
		if m.view != viewDisk {
			due *= 4 // /system/df is expensive: scan rarely when nobody looks at it
		}
		if m.hasDisk && !m.diskLoading && time.Since(m.disk.At) > due {
			return m, tea.Batch(m.diskCmd(), diskPoll())
		}
		return m, diskPoll()

	case diskMsg:
		m.diskLoading = false
		m.disk, m.hasDisk = collector.DiskReport(msg), true
		if msg.Err != nil {
			m.setStatus("disk scan: "+msg.Err.Error(), true)
		}
		m.rebuildDisk()
		if m.diskQueued {
			m.diskQueued = false
			return m, m.diskCmd()
		}
		return m, nil

	case eventMsg:
		return m.onEvent(events.Message(msg))

	case eventsErr:
		// daemon restarted or connection dropped: resubscribe after a pause
		ctx := m.ctx
		return m, func() tea.Msg {
			select {
			case <-time.After(3 * time.Second):
			case <-ctx.Done():
				return nil
			}
			return resubscribeMsg{}
		}

	case resubscribeMsg:
		m.subscribeEvents()
		return m, m.waitEvent()

	case actionMsg:
		if msg.err != nil {
			m.setStatus(msg.err.Error(), true)
		} else if msg.text != "" {
			m.setStatus(msg.text, false)
		}
		return m, tea.Batch(m.sampleCmd(), m.diskCmd())

	case logMsg:
		if m.logs != nil && m.logs.id == msg.id {
			return m, m.logs.update(msg)
		}
		return m, nil

	case tea.MouseMsg:
		return m.onMouse(msg)

	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

type resubscribeMsg struct{}

func (m Model) onEvent(e events.Message) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.waitEvent()}
	name := e.Actor.Attributes["name"]
	if name == "" {
		name = docker.ShortID(e.Actor.ID)
	}
	action := string(e.Action)
	// exec_* and health_status events are noisy; keep them out of the feed
	noisy := strings.HasPrefix(action, "exec_") || strings.HasPrefix(action, "health_status")
	if !noisy {
		m.evts = append(m.evts, eventLine{at: time.Unix(0, e.TimeNano), typ: string(e.Type), action: action, target: name})
		if len(m.evts) > 200 {
			m.evts = m.evts[len(m.evts)-200:]
		}
	}
	if e.Type == events.ContainerEventType {
		m.col.Invalidate(e.Actor.ID)
	}
	// anything that changes disk usage schedules a debounced df scan
	switch action {
	case "create", "destroy", "delete", "prune", "pull", "tag", "untag", "import", "load", "commit", "die":
		if !m.diskDebounce {
			m.diskDebounce = true
			cmds = append(cmds, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return diskTickMsg{} }))
		}
	}
	return m, tea.Batch(cmds...)
}

func (m Model) onKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.logs != nil {
		return m.logsKey(k)
	}
	if m.confirm != nil {
		return m.confirmKey(key)
	}
	if m.menu != nil {
		return m.menuKey(key)
	}
	if m.help {
		m.help = false
		return m, nil
	}
	if m.filtering {
		switch key {
		case "enter":
			m.filtering = false
			m.filter.Blur()
		case "esc":
			m.filtering = false
			m.filter.Blur()
			m.filter.SetValue("")
		default:
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(k)
			m.rebuildRes()
			m.rebuildDisk()
			return m, cmd
		}
		m.rebuildRes()
		m.rebuildDisk()
		return m, nil
	}

	switch key {
	case "q":
		return m, tea.Quit
	case "1":
		m.view = viewResources
		return m, nil
	case "2":
		m.view = viewDisk
		if !m.hasDisk || time.Since(m.disk.At) > m.opt.DiskInterval {
			return m, m.diskCmd()
		}
		return m, nil
	case "?", "h":
		m.help = true
		return m, nil
	case "/":
		m.filtering = true
		return m, m.filter.Focus()
	case "esc":
		// first esc drops the multi-selection, the next one clears the filter
		if sel := m.curSelector(); sel.hasMarks() {
			sel.clearMarks()
			return m, nil
		}
		m.filter.SetValue("")
		m.rebuildRes()
		m.rebuildDisk()
		return m, nil
	case "m":
		m.opt.Mode = 1 - m.opt.Mode
		return m, nil
	case "+", "=":
		m.opt.Interval = min(m.opt.Interval+500*time.Millisecond, 10*time.Second)
		m.setStatus("interval "+m.opt.Interval.String(), false)
		return m, nil
	case "-", "_":
		m.opt.Interval = max(m.opt.Interval-500*time.Millisecond, 500*time.Millisecond)
		m.setStatus("interval "+m.opt.Interval.String(), false)
		return m, nil
	case "ctrl+r", "f5":
		m.setStatus("refreshing…", false)
		return m, tea.Batch(m.sampleCmd(), m.diskCmd())
	}

	if m.view == viewDisk {
		return m.diskKey(key)
	}
	return m.resKey(key)
}

func (m Model) onMouse(ev tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.logs != nil {
		var cmd tea.Cmd
		m.logs.vp, cmd = m.logs.vp.Update(ev)
		m.logs.follow = m.logs.vp.AtBottom()
		return m, cmd
	}
	if m.confirm != nil || m.menu != nil {
		return m, nil
	}
	switch {
	case ev.Button == tea.MouseButtonWheelUp:
		m.moveSel(-3)
	case ev.Button == tea.MouseButtonWheelDown:
		m.moveSel(3)
	case ev.Action == tea.MouseActionPress && ev.Button == tea.MouseButtonLeft:
		m.clickRow(ev.Y)
	}
	return m, nil
}

func (m *Model) moveSel(d int) {
	if m.view == viewDisk {
		m.dsk.cur().move(d, m.diskLayout().rows)
	} else {
		m.res.selector.move(d, m.resLayout().rows)
	}
}

func (m *Model) curSelector() *selector {
	if m.view == viewDisk {
		return m.dsk.cur()
	}
	return &m.res.selector
}

// clickRow selects the table row under screen row y (and drops the multi-selection).
func (m *Model) clickRow(y int) {
	m.curSelector().clearMarks()
	if m.view == viewDisk {
		l := m.diskLayout()
		if r := y - l.tableTop; r >= 0 && r < l.rows {
			m.dsk.cur().click(r, l.rows)
		}
		return
	}
	l := m.resLayout()
	if r := y - l.tableTop; r >= 0 && r < l.rows {
		m.res.selector.click(r, l.rows)
	}
}

// execShell suspends the TUI and opens an interactive shell in the container
// through the docker CLI (needs a TTY, which the Engine API alone can't give us cheaply).
func (m *Model) execShell(id, name string) tea.Cmd {
	if _, err := exec.LookPath("docker"); err != nil {
		m.setStatus("exec needs the docker CLI in PATH", true)
		return nil
	}
	c := exec.Command("docker", "-H", m.cli.Endpoint, "exec", "-it", id, "sh", "-c",
		"command -v bash >/dev/null 2>&1 && exec bash || exec sh")
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{text: "left shell of " + name}
	})
}

var debug = os.Getenv("DTOP_DEBUG") != ""

// debugf logs to the DTOP_DEBUG file (set up by main with tea.LogToFile).
func debugf(format string, args ...any) {
	if debug {
		log.Printf(format, args...)
	}
}
