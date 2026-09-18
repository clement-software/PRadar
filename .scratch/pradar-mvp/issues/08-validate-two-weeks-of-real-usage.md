# 08: Validate two weeks of real usage

**What to build:** Collect the PRD's real-usage signals over two weeks of
daily use, locally, so the decision to continue rests on evidence rather than
on the corpus alone.

**Blocked by:** 07: Score the frozen corpus on the MVP interface.

**Status:** ready-for-human

- [ ] The application records, locally, the signals the PRD names: median time
  to comprehension, share of analyses judged useful, critical factual errors,
  share of pull requests understood without opening the Forgejo diff, and
  failures, replays and delays between a new version and its carte.
- [ ] Recording a signal is a deliberate user action or a measurement the
  application already owns; nothing is inferred from reading behaviour without
  the user knowing.
- [ ] The signals can be exported on demand and contain no credential or
  pull-request body.
- [ ] No measurement leaves the machine.
- [ ] After two weeks, the summary is promoted to `docs/product/PRD.md` with
  the decision it supports.
