package collector

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	"github.com/albedev/dtop/internal/docker"
	"github.com/albedev/dtop/internal/host"
)

// Usage says how an object is used, ordered from "most alive" to "garbage".
type Usage int

const (
	UsageActive   Usage = iota // used by a running container / build in progress
	UsageInUse                 // referenced by a stopped container
	UsageUnused                // not referenced, but tagged/named
	UsageDangling              // untagged image, anonymous unused volume
)

// Label renders the usage with wording that fits the object kind.
func (u Usage) Label(kind string) string {
	if kind == "containers" {
		return [...]string{"running", "in use", "created", "stopped"}[u]
	}
	if kind == "cache" {
		return [...]string{"building", "shared", "unused", "unused"}[u]
	}
	return [...]string{"active", "in use", "unused", "dangling"}[u]
}

// DiskItem is a row in one of the disk sections.
type DiskItem struct {
	ID       string
	Name     string
	Detail   string // image of a container, driver of a volume, type of a cache record
	Usage    Usage
	UsedBy   []string // container names
	Size     int64    // bytes attributed to this object
	Shared   int64    // bytes shared with other objects (images only)
	Created  time.Time
	LastUsed time.Time // zero = never / unknown
	Running  bool      // LastUsed is "now"
	Extra    string    // free text: tags count, usage count, ...
}

// Reclaimable reports whether deleting the item frees space without breaking a running workload.
func (d DiskItem) Reclaimable() bool { return d.Usage >= UsageUnused }

// Section is one category of docker disk usage.
type Section struct {
	Kind        string // images, containers, volumes, cache
	Total       int64
	Reclaimable int64
	Active      int
	Items       []DiskItem
}

// DiskReport is the result of a /system/df scan.
type DiskReport struct {
	At       time.Time
	Took     time.Duration
	Capacity host.DiskCapacity
	Used     int64 // sum of the sections, as `docker system df`
	Sections []Section
	Err      error
}

var anonVolume = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Disk runs /system/df (slow: seconds on big hosts) and derives usage/last-use.
// diskLimit overrides capacity detection (bytes, 0 = auto).
func (c *Collector) Disk(ctx context.Context, diskLimit int64) DiskReport {
	start := time.Now()
	rep := DiskReport{At: start}
	du, err := c.cli.DiskUsage(ctx, client.DiskUsageOptions{
		Containers: true, Images: true, Volumes: true, BuildCache: true, Verbose: true,
	})
	if err != nil {
		rep.Err = err
		return rep
	}

	// Container timing comes from inspect: cached by the resources sampler, fetched otherwise.
	type ctr struct {
		name           string
		running        bool
		started, ended time.Time
	}
	ctrs := map[string]ctr{}
	byImage := map[string][]string{}
	byVolume := map[string][]string{}
	for _, s := range du.Containers.Items {
		cs := ctr{name: docker.ContainerName(s.Names, s.ID), running: s.State == container.StateRunning}
		if in, ok := c.inspect(ctx, s); ok && in.State != nil {
			cs.started, cs.ended = parseTime(in.State.StartedAt), parseTime(in.State.FinishedAt)
		}
		ctrs[s.ID] = cs
		byImage[s.ImageID] = append(byImage[s.ImageID], s.ID)
		for _, m := range s.Mounts {
			if m.Name != "" && string(m.Type) == "volume" {
				byVolume[m.Name] = append(byVolume[m.Name], s.ID)
			}
		}
	}
	lastUse := func(ids []string) (names []string, running bool, last time.Time) {
		for _, id := range ids {
			cs := ctrs[id]
			names = append(names, cs.name)
			running = running || cs.running
			for _, t := range []time.Time{cs.started, cs.ended} {
				if t.After(last) {
					last = t
				}
			}
		}
		sort.Strings(names)
		return
	}

	// ---- images ----
	imgs := Section{Kind: "images", Total: du.Images.TotalSize, Reclaimable: du.Images.Reclaimable}
	for _, im := range du.Images.Items {
		names, running, last := lastUse(byImage[im.ID])
		it := DiskItem{
			ID: im.ID, Name: imageName(im.RepoTags, im.RepoDigests, im.ID),
			UsedBy: names, Size: im.Size, Shared: max(im.SharedSize, 0),
			Created: time.Unix(im.Created, 0), LastUsed: last, Running: running,
		}
		if len(im.RepoTags) > 1 {
			it.Extra = strings.Join(im.RepoTags[1:], " ")
		}
		switch {
		case running:
			it.Usage = UsageActive
		case len(names) > 0:
			it.Usage = UsageInUse
		case len(im.RepoTags) == 0:
			it.Usage = UsageDangling
		default:
			it.Usage = UsageUnused
		}
		if it.Usage == UsageActive {
			imgs.Active++
		}
		imgs.Items = append(imgs.Items, it)
	}

	// ---- containers ----
	cons := Section{Kind: "containers", Total: du.Containers.TotalSize, Reclaimable: du.Containers.Reclaimable}
	for _, s := range du.Containers.Items {
		cs := ctrs[s.ID]
		it := DiskItem{
			ID: s.ID, Name: cs.name, Detail: s.Image, Size: s.SizeRw,
			Created: time.Unix(s.Created, 0), Running: cs.running, UsedBy: nil,
			Extra: s.Status,
		}
		it.LastUsed = cs.ended
		if cs.started.After(it.LastUsed) {
			it.LastUsed = cs.started
		}
		switch s.State {
		case container.StateRunning, container.StatePaused, container.StateRestarting:
			it.Usage = UsageActive
			cons.Active++
		case container.StateCreated:
			it.Usage = UsageUnused
		default:
			it.Usage = UsageDangling // stopped container: removable writable layer
		}
		cons.Items = append(cons.Items, it)
	}

	// ---- volumes ----
	vols := Section{Kind: "volumes", Total: du.Volumes.TotalSize, Reclaimable: du.Volumes.Reclaimable}
	for _, v := range du.Volumes.Items {
		names, running, last := lastUse(byVolume[v.Name])
		it := DiskItem{
			ID: v.Name, Name: v.Name, Detail: v.Driver, UsedBy: names,
			Size: -1, LastUsed: last, Running: running,
		}
		if v.UsageData != nil {
			it.Size = v.UsageData.Size
		}
		if t, err := time.Parse(time.RFC3339, v.CreatedAt); err == nil {
			it.Created = t
		}
		anonymous := anonVolume.MatchString(v.Name) || v.Labels["com.docker.volume.anonymous"] != ""
		if anonymous {
			it.Extra = "anonymous"
		}
		switch {
		case running:
			it.Usage = UsageActive
			vols.Active++
		case len(names) > 0:
			it.Usage = UsageInUse
		case anonymous:
			it.Usage = UsageDangling
		default:
			it.Usage = UsageUnused
		}
		vols.Items = append(vols.Items, it)
	}

	// ---- build cache ----
	cache := Section{Kind: "cache", Total: du.BuildCache.TotalSize, Reclaimable: du.BuildCache.Reclaimable}
	for _, b := range du.BuildCache.Items {
		it := DiskItem{
			ID: b.ID, Name: cacheName(b.Description, b.ID), Detail: b.Type,
			Size: b.Size, Created: b.CreatedAt,
		}
		if b.LastUsedAt != nil {
			it.LastUsed = *b.LastUsedAt
		}
		if b.UsageCount > 0 {
			it.Extra = pluralize(b.UsageCount, "use")
		}
		if b.Shared {
			it.Extra = strings.TrimSpace(it.Extra + " shared")
		}
		switch {
		case b.InUse:
			it.Usage = UsageActive
			it.Running = true
			cache.Active++
		case b.Shared:
			it.Usage = UsageInUse // shared with an image: pruning it frees nothing
		default:
			it.Usage = UsageUnused
		}
		cache.Items = append(cache.Items, it)
	}

	rep.Sections = []Section{imgs, cons, vols, cache}
	for _, s := range rep.Sections {
		rep.Used += s.Total
	}
	rep.Capacity = host.DetectDisk(host.Options{
		Local: c.cli.Local, RootDir: c.info.RootDir, OperatingOS: c.info.OS, Override: diskLimit,
	})
	rep.Took = time.Since(start)
	return rep
}

func (c *Collector) inspect(ctx context.Context, s container.Summary) (container.InspectResponse, bool) {
	c.mu.Lock()
	ic, ok := c.inspects[s.ID]
	c.mu.Unlock()
	if ok {
		return ic.resp, true
	}
	r, err := c.cli.ContainerInspect(ctx, s.ID, client.ContainerInspectOptions{})
	if err != nil {
		return container.InspectResponse{}, false
	}
	return r.Container, true
}

func imageName(tags, digests []string, id string) string {
	if len(tags) > 0 && tags[0] != "<none>:<none>" {
		return tags[0]
	}
	if len(digests) > 0 {
		if i := strings.Index(digests[0], "@"); i > 0 {
			return digests[0][:i] + "@" + docker.ShortID(digests[0][i+1:])
		}
	}
	return "<none>@" + docker.ShortID(id)
}

func cacheName(desc, id string) string {
	desc = strings.TrimSpace(desc)
	if desc == "" {
		return docker.ShortID(id)
	}
	return desc
}

func pluralize(n int, word string) string {
	s := strconv.Itoa(n) + " " + word
	if n != 1 {
		s += "s"
	}
	return s
}
