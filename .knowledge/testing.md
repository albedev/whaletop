# Testing

## Unit test
`make test` (= `go test ./...`). Covers: CPU/mem/rate formulas and counter resets (`collector/resources_test.go`),
immutability of histories, block/tty glyphs and absence of braille, box/fit/overlay widths, `short()` always with units,
selector that follows the ID after a re-sort (`ui/widgets_test.go`).

## Render without TTY
`./bin/whaletop --dump 160x45` (or `make dump`): two samples + one disk scan, prints one frame per view.
With `CLICOLOR_FORCE=1` it keeps ANSI colors. Great for checking layout at various widths (e.g. 90x30, 100x30).

## Real TUI driven (as done in development)
```
tmux new-session -d -s dt -x 150 -y 42 ./bin/whaletop
tmux send-keys -t dt Down k        # select and ask kill → the confirmation appears
tmux capture-pane -t dt -p         # text screenshot
tmux attach -t dt                  # to watch it live (ctrl+b d to detach)
```

## Debug log
`WHALETOP_DEBUG=/tmp/whaletop.log ./bin/whaletop` → every Update message is logged (`debugf`). Useful for loop/event problems.

## Fixtures used
```
docker volume create whaletop-test-data
docker run -d --name whaletop-test-web -p 18080:80 -l com.docker.compose.project=whaletoptest nginx:alpine
docker run -d --name whaletop-test-redis --memory 256m --cpus 0.5 -v whaletop-test-data:/data -l com.docker.compose.project=whaletoptest redis:7-alpine
docker run -d --name whaletop-test-burn --cpus 1.5 alpine:3.20 sh -c 'while :; do :; done'         # ~100% of one core
docker run -d --name whaletop-test-mem --memory 128m alpine:3.20 sh -c 'dd if=/dev/zero of=/dev/shm/x bs=1M count=80; sleep infinity'
docker run --name whaletop-test-exited -v /anon alpine:3.20 sh -c 'echo hello; echo oops >&2; exit 3'   # exited(3) + anonymous volume
# 2 tagged images + 1 dangling + build cache: two Dockerfiles FROM alpine with RUN dd …, then build --no-cache without tag
```
Cleanup: `docker rm -f $(docker ps -aq --filter name=whaletop-test-) ; docker volume rm whaletop-test-data ; docker rmi whaletop-test-img:1 whaletop-test-img:2 ; docker builder prune -af`.
