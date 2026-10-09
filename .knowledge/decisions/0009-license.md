# ADR 0009 — License: MIT No-Sale (source-available)
Date: 2026-10-08 · Status: accepted (user request)

**User requirements.** A very permissive license (forks, derivatives, contributions); companies may use it;
selling the software for a fee is forbidden; **no derivative may be distributed for a fee**.

**Discarded alternatives.**
- Plain MIT / Apache-2.0: they permit sale.
- MIT + Commons Clause v1.0: prohibits "selling" the software, but the official FAQ states that **derivatives that add significant value may be sold** → violates the requirement on derivatives.
- PolyForm Noncommercial 1.0: also prohibits internal commercial use by companies → too restrictive.
- CC BY-NC-SA 4.0: Creative Commons advises against its licenses for software; "NC" is ambiguous for corporate use.

**Decision.** `LICENSE` = modified MIT ("MIT No-Sale v1.0"): MIT grant without "sublicense"/"sell"; definitions of
Derivative Work and Sell (sale of copies/licenses/subscriptions, bundles sold, paid hosted services based on whaletop);
conditions: No Sale for all recipients, derivatives only under the same license and free with sources, notice,
contributions under the same license, third-party libraries under their own licenses; violation = termination of rights.

**Consequences.**
- It is not "open source" according to the OSI: it must be called *source-available*. Some distros/registries (e.g. Debian main, homebrew-core)
  may not accept it.
- Custom license: not reviewed by a lawyer. If the project becomes significant, have it reviewed.
- Third-party libraries (MIT/Apache/BSD) remain under their own licenses (`THIRD_PARTY_NOTICES.md`).
- Paid consulting/support is *not* explicitly prohibited (only paid hosting): a deliberate choice to keep corporate
  use free; can be changed in a v1.1 if the user wishes.
