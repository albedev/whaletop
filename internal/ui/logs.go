package ui

import (
	"bufio"
	"context"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
)

const maxLogLines = 5000

type logMsg struct {
	id    string
	lines []string
	err   error
	done  bool
}

type logView struct {
	id, name string
	vp       viewport.Model
	lines    []string
	follow   bool
	ended    bool
	ch       chan logMsg
	cancel   context.CancelFunc
}

func (m *Model) openLogs(id, name string, _ bool) tea.Cmd {
	ctx, cancel := context.WithCancel(m.ctx)
	lv := &logView{id: id, name: name, follow: true, ch: make(chan logMsg, 16), cancel: cancel}
	lv.vp = viewport.New(m.w-2, max(m.h-4, 1))
	lv.resize(m.w, m.h)
	m.logs = lv
	go streamLogs(ctx, m.cli.Client, id, lv.ch)
	return lv.wait()
}

func (l *logView) wait() tea.Cmd {
	ch := l.ch
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

func (l *logView) resize(w, h int) {
	l.vp.Width = max(w-2, 1)
	l.vp.Height = max(h-4, 1)
}

func (l *logView) update(msg logMsg) tea.Cmd {
	l.lines = append(l.lines, msg.lines...)
	if msg.err != nil {
		l.lines = append(l.lines, sRed.Render("── "+msg.err.Error()))
	}
	if len(l.lines) > maxLogLines {
		l.lines = l.lines[len(l.lines)-maxLogLines:]
	}
	l.vp.SetContent(strings.Join(l.lines, "\n"))
	if l.follow {
		l.vp.GotoBottom()
	}
	if msg.done {
		l.ended = true
		return nil
	}
	return l.wait()
}

// streamLogs follows container logs and sends batches of lines every 100ms.
func streamLogs(ctx context.Context, cli *client.Client, id string, ch chan<- logMsg) {
	defer close(ch)
	tty := false
	if in, err := cli.ContainerInspect(ctx, id, client.ContainerInspectOptions{}); err == nil && in.Container.Config != nil {
		tty = in.Container.Config.Tty
	}
	rc, err := cli.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: true, Tail: "500",
	})
	if err != nil {
		ch <- logMsg{id: id, err: err, done: true}
		return
	}
	defer rc.Close()

	lines := make(chan string, 1024)
	outR, outW := io.Pipe()
	errR, errW := io.Pipe()
	scan := func(r io.Reader, stderr bool, done chan<- struct{}) {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		for sc.Scan() {
			t := strings.ReplaceAll(sc.Text(), "\t", "    ")
			if stderr {
				t = sRed.Render(t)
			}
			select {
			case lines <- t:
			case <-ctx.Done():
			}
		}
		_, _ = io.Copy(io.Discard, r) // keep draining if a line was too long
		done <- struct{}{}
	}
	done := make(chan struct{}, 2)
	go scan(outR, false, done)
	go scan(errR, true, done)
	go func() {
		var err error
		if tty {
			_, err = io.Copy(outW, rc) // TTY streams are not multiplexed
		} else {
			_, err = stdcopy.StdCopy(outW, errW, rc)
		}
		outW.CloseWithError(err)
		errW.CloseWithError(err)
		<-done
		<-done
		close(lines)
	}()

	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	var batch []string
	for {
		select {
		case l, ok := <-lines:
			if !ok {
				ch <- logMsg{id: id, lines: batch, done: true}
				return
			}
			batch = append(batch, l)
		case <-t.C:
			if len(batch) > 0 {
				select {
				case ch <- logMsg{id: id, lines: batch}:
					batch = nil
				case <-ctx.Done():
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (m Model) logsKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	l := m.logs
	switch k.String() {
	case "esc", "q", "l":
		l.cancel()
		m.logs = nil
		return m, nil
	case "f":
		l.follow = !l.follow
		if l.follow {
			l.vp.GotoBottom()
		}
		return m, nil
	case "G", "end":
		l.vp.GotoBottom()
		l.follow = true
		return m, nil
	case "g", "home":
		l.vp.GotoTop()
		l.follow = false
		return m, nil
	}
	var cmd tea.Cmd
	l.vp, cmd = l.vp.Update(k)
	l.follow = l.vp.AtBottom()
	return m, cmd
}

func (m Model) renderLogs() string {
	l := m.logs
	state := sGreen.Render(" follow")
	if !l.follow {
		state = sYel.Render(" paused")
	}
	if l.ended {
		state = sDim.Render(" ended")
	}
	title := sTitle.Render("logs ") + sCyan.Render(l.name) + state + sDim.Render(" · esc close · f follow · g/G top/bottom")
	body := strings.Split(l.vp.View(), "\n")
	return box(title, body, m.w, m.h-1, cBoxProc)
}
