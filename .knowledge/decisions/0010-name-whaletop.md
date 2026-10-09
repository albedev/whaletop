# ADR 0010 — Name: whaletop (alias wtop)
Date: 2026-10-09 · Status: accepted (user choice)

**Context.** The project was developed as `dtop`, but before publishing we found that `dtop` is taken:
amir20/dtop (Rust, MIT) ships as the `dtop` formula in homebrew-core (0.9.4 at the time), so `brew install dtop`
installs that tool and the two binaries would clash on PATH.

**Options checked (2026-10-09, Homebrew core/cask + GitHub repos with the exact name).**
- `whaletop`: free on Homebrew, 2 GitHub repos with 0 stars. Whale = Docker's mascot, `top` = top/htop/btop family.
- `keeltop`, `hulltop`: completely free, but less self-explanatory.
- `wtop`: free on Homebrew, but taken by ClockworkNet/wtop ("top for Apache", 92★, published on PyPI as `wtop`)
  and by several htop clones for Windows (where "w" = Windows): ambiguous and misleading.
- Names containing "docker" (dockertop, dockscope) were avoided: Docker trademark guidelines discourage product names
  that suggest an official Docker product.

**Decision.** Name and binary **whaletop**, module `github.com/albedev/whaletop`, plus the short alias **`wtop`**
(requested by the user) provided as a symlink by `make install` and by the Homebrew formula
(`bin.install_symlink "whaletop" => "wtop"`). The alias is never forced over an existing `wtop` that is not ours.
`go install` users only get `whaletop` (Go installs one binary per main package); the README says how to alias it.

**Consequences.** Debug env var is `WHALETOP_DEBUG`. The GitHub repo was renamed `albedev/dtop` → `albedev/whaletop` by the owner
(GitHub redirects the old URL). The local working directory may still be called `dtop`: it does not matter.
