# Captcha Plugin Protocol

The join captcha ships with built-in questions, but this server is open source,
so those questions are public. A plugin replaces them with your own: the server
stops generating anything, asks your program for a question, shows it, and
passes the player's answer back. What the question is, how it is built and how
it is checked live in a binary that is not in this repository.

The public source then shows only *that* a captcha exists and *where* it
attaches — which was never the part worth keeping secret.

```toml
captcha_plugin = "/opt/nyathena/captcha-plugin"
captcha_plugin_timeout = 3000   # milliseconds
```

The plugin is a normal executable in any language. It is **not** linked into the
server, so it is not bound by the server's AGPL licence, it cannot crash the
server, and it does not have to be rebuilt when the server's Go version changes.

## How it runs

The server starts your program once at boot and keeps it running, restarting it
with backoff if it exits. Requests go to its **stdin**, one JSON object per
line. Responses go to its **stdout**, one JSON object per line.

Anything written to **stderr** is copied into the server log, prefixed with the
plugin name. That is the intended way to log.

> Write nothing but protocol lines to stdout, or the server cannot parse them.

Responses may be returned in any order — each is matched by its `id` — so you
are free to answer concurrently.

## Requests

```json
{"id": 1, "op": "challenge", "data": {"ipid": "a1b2c3", "uid": 7, "area": "Courtroom"}}
{"id": 2, "op": "verify",    "data": {"ipid": "a1b2c3", "uid": 7, "token": "…", "answer": "12"}}
```

| Op | When | Fields in `data` |
|----|------|------------------|
| `challenge` | A new IPID joined and needs a question | `ipid`, `uid`, `area` |
| `verify` | The player answered a token-mode challenge | `ipid`, `uid`, `token`, `answer` |

## Responses

Echo the request's `id`. Put your payload in `data`, or set `error` to report a
failure.

### `challenge`

```json
{"id": 1, "data": {
  "prompt":  "Count the vowels in: harbour lantern",
  "hint":    "Reply with:  /verify <your answer>",
  "kind":    "vowelcount",
  "token":   "opaque-state-you-choose",
  "answers": ["5", "five"]
}}
```

| Field | Required | Meaning |
|-------|----------|---------|
| `prompt` | yes | The question, shown in OOC and in the client popup |
| `hint` | no | How to answer; defaults to `Reply with:  /verify <your answer>` |
| `kind` | no | A label for staff output and logs; defaults to `plugin` |
| `answers` | see below | Accepted answers, compared case/punctuation-insensitively |
| `token` | see below | Opaque state handed back on `verify` |

You must supply **`answers`, `token`, or both**. A response with neither is
rejected and a built-in question is used instead, because the player could never
pass it.

**Answers mode** — return `answers`. The server checks locally, so a wrong guess
costs no round trip. Use this when the value you are adding is the questions.

**Token mode** — return `token` and no `answers`. The server calls `verify` for
every attempt and you alone decide. In this mode the answer never exists inside
the server process at all, so a memory dump of the server reveals nothing.

### `verify`

```json
{"id": 2, "data": {"ok": true}}
```

## Failure handling

Design your plugin knowing the server will keep going without it:

- **Plugin down, or slow, on `challenge`** → the server uses a built-in question.
  This is deliberate. A captcha that stops working because a helper crashed
  would open the gate during exactly the incident it exists for.
- **Plugin down, or slow, on `verify`** → the player is **not** counted as
  wrong. They may well have answered correctly, and quarantining a real player
  over your process's outage is the false positive this whole feature has to
  avoid. They are handed a fresh built-in question instead.
- **`error` in a response** → logged, then treated as above.

## One rule worth keeping

Whatever questions you write, keep the property the built-in ones have:

> **The answer must never appear in the question.**

"Type `A7K9` to verify" is defeated by three lines of regex, no matter how
random `A7K9` is. Make the answer something that has to be *derived* — a sum, a
reversal, a count, a selection — so a scraper has nothing to copy.

## Minimal example

```python
#!/usr/bin/env python3
import json, random, sys

def challenge():
    a, b = random.randint(2, 9), random.randint(2, 9)
    words = ["zero","one","two","three","four","five","six","seven","eight","nine"]
    return {
        "prompt": f"What is {words[a]} plus {words[b]}?",
        "kind": "sum",
        "answers": [str(a + b)],
    }

for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    req = json.loads(line)
    if req.get("op") == "challenge":
        resp = {"id": req["id"], "data": challenge()}
    else:
        resp = {"id": req["id"], "error": "unsupported op"}
    print(json.dumps(resp), flush=True)     # flush matters: the server is waiting
```

Keep your real one out of any public repository — that is the entire point.
