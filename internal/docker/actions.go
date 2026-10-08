package docker

import (
	"context"
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/moby/moby/client"
)

// Action identifiers used by the UI. Destructive ones must be confirmed by the user.
type Action string

const (
	ActStart   Action = "start"
	ActStop    Action = "stop"
	ActRestart Action = "restart"
	ActKill    Action = "kill"
	ActPause   Action = "pause"
	ActUnpause Action = "unpause"
	ActRemove  Action = "remove" // container / image / volume / cache record
	ActForce   Action = "force-remove"

	ActPruneContainers Action = "prune-containers"
	ActPruneDangling   Action = "prune-dangling-images"
	ActPruneImages     Action = "prune-unused-images"
	ActPruneAnonVols   Action = "prune-anonymous-volumes"
	ActPruneVolumes    Action = "prune-unused-volumes"
	ActPruneCache      Action = "prune-build-cache"
)

// Destructive reports whether an action loses data and needs confirmation.
func (a Action) Destructive() bool {
	switch a {
	case ActStart, ActStop, ActRestart, ActPause, ActUnpause:
		return false
	}
	return true
}

const stopTimeout = 10 // seconds, same default as `docker stop`

// ContainerAction runs a lifecycle action on a container.
func (c *Client) ContainerAction(ctx context.Context, id string, a Action) error {
	var err error
	t := stopTimeout
	switch a {
	case ActStart:
		_, err = c.ContainerStart(ctx, id, client.ContainerStartOptions{})
	case ActStop:
		_, err = c.ContainerStop(ctx, id, client.ContainerStopOptions{Timeout: &t})
	case ActRestart:
		_, err = c.ContainerRestart(ctx, id, client.ContainerRestartOptions{Timeout: &t})
	case ActKill:
		_, err = c.ContainerKill(ctx, id, client.ContainerKillOptions{Signal: "SIGKILL"})
	case ActPause:
		_, err = c.ContainerPause(ctx, id, client.ContainerPauseOptions{})
	case ActUnpause:
		_, err = c.ContainerUnpause(ctx, id, client.ContainerUnpauseOptions{})
	case ActRemove:
		_, err = c.ContainerRemove(ctx, id, client.ContainerRemoveOptions{})
	case ActForce:
		_, err = c.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})
	default:
		err = fmt.Errorf("unsupported container action %q", a)
	}
	return err
}

// RemoveObject deletes an item of a disk section ("images", "containers", "volumes", "cache").
func (c *Client) RemoveObject(ctx context.Context, kind, id string, force bool) (string, error) {
	switch kind {
	case "containers":
		a := ActRemove
		if force {
			a = ActForce
		}
		return "", c.ContainerAction(ctx, id, a)
	case "images":
		_, err := c.ImageRemove(ctx, id, client.ImageRemoveOptions{Force: force, PruneChildren: true})
		return "", err
	case "volumes":
		_, err := c.VolumeRemove(ctx, id, client.VolumeRemoveOptions{Force: force})
		return "", err
	case "cache":
		r, err := c.BuildCachePrune(ctx, client.BuildCachePruneOptions{
			All: true, Filters: make(client.Filters).Add("id", id),
		})
		if err != nil {
			return "", err
		}
		if len(r.Report.CachesDeleted) == 0 {
			return "", fmt.Errorf("cache record not removed (in use or shared with an image)")
		}
		return "freed " + humanize.IBytes(r.Report.SpaceReclaimed), nil
	}
	return "", fmt.Errorf("unknown kind %q", kind)
}

// Prune runs one of the prune actions and returns a human summary.
func (c *Client) Prune(ctx context.Context, a Action) (string, error) {
	var n int
	var freed uint64
	switch a {
	case ActPruneContainers:
		r, err := c.ContainerPrune(ctx, client.ContainerPruneOptions{})
		if err != nil {
			return "", err
		}
		n, freed = len(r.Report.ContainersDeleted), r.Report.SpaceReclaimed
	case ActPruneDangling, ActPruneImages:
		f := make(client.Filters).Add("dangling", "true")
		if a == ActPruneImages {
			f = make(client.Filters).Add("dangling", "false")
		}
		r, err := c.ImagePrune(ctx, client.ImagePruneOptions{Filters: f})
		if err != nil {
			return "", err
		}
		for _, d := range r.Report.ImagesDeleted {
			if d.Deleted != "" { // entries are either "Untagged" or "Deleted"
				n++
			}
		}
		freed = r.Report.SpaceReclaimed
	case ActPruneAnonVols, ActPruneVolumes:
		r, err := c.VolumePrune(ctx, client.VolumePruneOptions{All: a == ActPruneVolumes})
		if err != nil {
			return "", err
		}
		n, freed = len(r.Report.VolumesDeleted), r.Report.SpaceReclaimed
	case ActPruneCache:
		r, err := c.BuildCachePrune(ctx, client.BuildCachePruneOptions{All: true})
		if err != nil {
			return "", err
		}
		n, freed = len(r.Report.CachesDeleted), r.Report.SpaceReclaimed
	default:
		return "", fmt.Errorf("unsupported prune %q", a)
	}
	return fmt.Sprintf("%s: %d removed, %s freed", a, n, humanize.IBytes(freed)), nil
}
