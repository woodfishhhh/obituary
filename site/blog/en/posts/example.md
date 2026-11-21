---
title: 'Example: marking is not burial'
description: 'Two state changes in the life of one file, and what the ledger can tell you after each.'
developmentDate: '2026-11-21'
translationKey: example
draft: true
---

After a file receives a lifespan, it stays in its original directory. The ledger can tell you when it is due, but there is nothing to restore yet. The grave acquires its contents only when burial happens.

That distinction determines what to verify: check the deadline and original location after registration, then check the grave record after burial. See the [architecture contract](https://github.com/woodfishhhh/obituary/blob/main/docs/architecture.md).
