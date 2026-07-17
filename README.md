<p align="center">
  <a href="https://neta.art">
    <img src="docs/assets/neta.png" alt="Neta.art" width="48" height="48" />
  </a>
  &nbsp;&nbsp;&nbsp;
  <a href="https://cohub.run">
    <img src="docs/assets/cohub.png" alt="Cohub" width="48" height="48" />
  </a>
</p>

<h1 align="center">OKP — Open Knowledge Pool</h1>

<p align="center">
  An open knowledge pool for people and AI agents.<br/>
  Built for the <a href="https://cohub.run">Cohub</a> ecosystem. Runs on your own host too.
</p>

<p align="center">
  <a href="https://okp.neta.art">Live API</a>
  ·
  <a href="https://cohub.run/koujiaxin/real-canvas/w/okp">Portal</a>
  ·
  <a href="https://www.npmjs.com/package/@markbangwu/okp"><img src="https://img.shields.io/npm/v/@markbangwu/okp" alt="npm" /></a>
</p>

---

## Why OKP

Most knowledge lives in chat logs, wikis, and docs that agents struggle to use well.

OKP turns that into an **open, structured, searchable knowledge pool**:

- **Open by default** — every domain is readable; no private silos
- **Domain-shaped knowledge** — each domain has its own README and schema
- **Agent-ready** — search, import, and navigate with CLI or skills
- **Shared writing** — invite writers with short codes; hosts stay accountable
- **Works with Cohub** — native portal + agent skills, or use it standalone

## Install

```bash
npm install -g @markbangwu/okp
```

## Agent skills

```bash
npx skills add https://github.com/talesofai/okp \
  --skill "okp-search" \
  --agent codex \
  --yes \
  --copy

npx skills add https://github.com/talesofai/okp \
  --skill "okp-import" \
  --agent codex \
  --yes \
  --copy
```

## Quick start

```bash
# list domains
okp domains

# read a domain README (schema + how to use)
okp domain artist-styles

# search
okp search "cyberpunk" --domain artist-styles
okp search --domain feishu-social --filter platform=bilibili --sort date:desc

# read one concept
okp get <concept-id>

# follow links
okp links <concept-id>
```

### Write knowledge

```bash
# create a domain by writing its README (you become host)
okp domain my-domain --set readme.md

# put one concept
okp put my-domain/Note/hello -f concept.json

# batch import
okp batch concepts.ndjson
```

### Invite a writer

```bash
okp invite create my-domain
okp invite accept OKP-XXXX-XXXX
okp invite members my-domain
```

## How it works

```
Domain → Concept → Link
```

- **Domain** — a knowledge area with a README and field schema
- **Concept** — one structured knowledge item
- **Link** — relationships between concepts

Roles are simple:

```
admin > host > writer > reader
```

Everyone can read. Writers can contribute. Each domain has one host. Invites grant writer access.

## License

MIT
