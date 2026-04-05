---
type: note
title: Implementation Log
created: 2026-04-02
tags:
  - implementation-log
  - chat
---

# Implementation Log

## 2026-04-06

- Scope: fixed installer raw GitHub URLs to use the actual default branch case on upstream remote.
- Files or subsystems touched: `README.md`, `README.id.md`, `web/src/views/AboutView.svelte`.
- Behavior/runtime effect: update/install curl commands now fetch from `https://raw.githubusercontent.com/zcuss/CodexSess/Main/scripts/install.sh` and no longer return 404 due to branch-case mismatch.
- Validation status: validated `curl -I` against raw URL (`/Main/`) returns HTTP 200 and `/main/` returns HTTP 404.
- Open follow-up items: none.

## 2026-04-06

- Scope: aligned updater/runtime GitHub repository references with the current remote owner.
- Files or subsystems touched: `internal/httpapi/server_update.go`, `web/src/views/AboutView.svelte`, `scripts/install.sh`.
- Behavior/runtime effect: update checking and update command generation now target `zcuss/CodexSess`, including installer default `--repo`.
- Validation status: verified no remaining `zcuss/CodexSess` references with ripgrep and reviewed git diff.
- Open follow-up items: none.

## 2026-04-06

- Scope: updated README GitHub links to match the current repository remote ownership.
- Files or subsystems touched: `README.md`, `README.id.md`.
- Behavior/runtime effect: release, badge, and installer-script links now point to `zcuss/CodexSess` instead of `zcuss/CodexSess`.
- Validation status: verified link replacements with `rg` and reviewed changed docs diff.
- Open follow-up items: none.

## 2026-04-02

- Scope: normalized the coding workspace to the current chat-only system snapshot.
- Files or subsystems touched: coding session storage and schema reset, HTTP/websocket session contracts, runtime debug payloads, frontend session display state, embedded web assets, regression coverage, and release/docs metadata.
- Behavior/runtime effect: `/chat` now runs as a single chat-first coding workspace, exposes one public `thread_id`, and drops legacy coding-session rows when an outdated schema is encountered.
- Validation status: `rtk timeout 120s go test ./...` passed; `cd web && rtk timeout 120s npm run test:unit && npm run build:web` passed.
- Open follow-up items: none.
