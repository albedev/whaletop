# ADR 0005 — Internal chart renderer instead of ntcharts
Date: 2026-10-08 · Status: accepted

Evaluated `NimbleMarkets/ntcharts` (MIT, for bubbletea). Not suitable because:
- styling is applied per chart, not per **row**: no btop-style vertical gradient;
- no `░▒▓█` mode (tty);
- the streamline chart relies on braille/arc runes, which the user does not want.
Our renderer (`ui/widgets.go`: graph, sparkline, meter, stackedBar) is ~150 lines, tested, with no extra dependencies.
If more complex charts are needed in the future (axes, legends, zoom), re-evaluate ntcharts.
