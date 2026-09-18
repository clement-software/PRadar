# 09: Show what is waiting to be analysed

**What to build:** Make the anti-rebond visible. A pending count alone reads as
a failure: the interface must say which pull requests are waiting, when their
analysis is due, and let the user reach them in Forgejo meanwhile.

**Blocked by:** 04: Build the production reading interface.

**Status:** done

- [x] The timeline lists every version waiting for analysis, with its
  repository, number, title, author and its Forgejo link.
- [x] Each waiting version says when its analysis is due, or that it is
  running now, so waiting is never mistaken for a failure.
- [x] A version being retried after a technical failure says which attempt is
  next and when.
- [x] The status strip says when the next analysis is due while anything is
  waiting.
- [x] A waiting version is plainly not a carte: it offers no analysis to open
  and disappears from the waiting list once its analysis is published.
- [x] The waiting list survives a restart and needs no background refresh to
  be correct when the page is loaded.
- [x] Acceptance tests cover a waiting version, a running one, a retried one
  and the list emptying once the analysis is published.
- [x] `make verify` passes.
