# ADR 0004 — "block" and "tty" charts, no braille
Date: 2026-10-08 · Status: accepted (user request)

The user: "no braille per favore, voglio blocks o tty stile btop" (no braille please, I want blocks or tty in btop style).
btop offers three `graph_symbol` values: braille, block, tty. whaletop implements only **block** (`▁▂▃▄▅▆▇█`, default) and **tty** (`░▒▓█`).
Selection: flag `--graph block|tty`, key `m` at runtime (always visible in the footer).
A test (`TestGraphShape`) verifies that no braille character appears in the charts. Do not reintroduce braille.
