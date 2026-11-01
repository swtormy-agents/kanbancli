# kanbancli API scope (v0.1)

This document lists REST endpoints used by **kanbancli v0.1**. The CLI is unofficial and not affiliated with or endorsed by PLANKA Software GmbH. Paths were identified from public HTTP behaviour of PLANKA-compatible servers; no PLANKA source code or documentation is included in this repository.

Authentication for authenticated calls: header `Authorization: Bearer <token>` from env `KANBANCLI_TOKEN` only. Base URL from env `KANBANCLI_URL` or flag `--url` (http/https, host required, no embedded credentials).

## Covered in v0.1

| Command | Method | Path |
|---------|--------|------|
| `kanbancli health` | GET | `/api/users/me` |
| `kanbancli version` | GET | `/api/config` |
| `kanbancli me` | GET | `/api/users/me` |
| `kanbancli auth login` | POST | `/api/access-tokens` |
| `kanbancli projects list` | GET | `/api/projects` |
| `kanbancli project get --id` | GET | `/api/projects/{id}` |
| `kanbancli boards get --id` | GET | `/api/boards/{id}` |
| `kanbancli lists list --board` | GET | `/api/boards/{id}` (lists extracted from response) |
| `kanbancli cards list --board` | GET | `/api/boards/{id}` (cards extracted from response) |
| `kanbancli card get --id` | GET | `/api/cards/{id}` |
| `kanbancli card create --list --name` | POST | `/api/lists/{list}/cards` |
| `kanbancli card move --id --list` | PATCH | `/api/cards/{id}` |
| `kanbancli card update --id` | PATCH | `/api/cards/{id}` |
| `kanbancli comments list --card` | GET | `/api/cards/{card}/comments` |
| `kanbancli comment add --card --text` | POST | `/api/cards/{card}/comments` |
| `kanbancli users list` | GET | `/api/users` (response limited to id, username, name) |
| `kanbancli api` | * | `/api/...` (query via `--query key=value`) |

Path segments `{id}`, `{list}`, `{card}`, and `{board}` are positive base-10 integers formatted into the URL (snowflake IDs on the server are numeric strings).

## Out of scope

- Enterprise or paid API routes (including paths containing `.pe.`)
- Returning user email addresses from `users list`
- Non-interactive writes (agents must not pass `--write`)

Use of a PLANKA server is governed by PLANKA's own licence; commercial or hosted deployments may require a separate PLANKA Pro/Enterprise licence from PLANKA Software GmbH.
