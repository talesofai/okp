<p align="center">
    <img src="docs/assets/neta.png" alt="Neta.art" width="48" height="48" />
    &nbsp;&nbsp;&nbsp;
    <img src="docs/assets/cohub.png" alt="Cohub" width="48" height="48" />
</p>

<h1 align="center">OKP — Open Knowledge Protocol</h1>

<p align="center">
  The <strong>context layer</strong> for AI agents — an open, structured knowledge protocol<br/>
  for people and agents. Built for the <a href="https://cohub.run">Cohub</a> ecosystem.<br/>
  Runs on your own host too.
</p>

<p align="center">
  <a href="https://okp.neta.art">Live API</a>
  ·
  <a href="https://cohub.run/koujiaxin/real-canvas/w/okp">Portal</a>
</p>

---

## Why OKP

Agents don't need more memory — they need better **context**. Most knowledge
lives in chat logs, wikis, and docs that agents struggle to use well.

OKP turns that into an **open, structured, searchable knowledge protocol** that
gives agents real context:

- **Public by default** — domains are open unless their creator chooses private visibility
- **Domain-shaped knowledge** — each domain has its own README and schema
- **Agent-ready** — search, import, and navigate with CLI or skills
- **Shared writing** — invite writers with short codes; hosts stay accountable
- **Works with Cohub** — native portal + agent skills, or use it standalone

## Install

```bash
npm install -g @markbangwu/okp
```

## Agent skills

Skills-first: OKP integrates through agent skills — search and import knowledge
straight from your agent, no MCP server needed.

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

# create a private domain
okp domain my-private-domain --set readme.md --visibility private

# put one concept
okp put my-domain/Note/hello -f concept.json

# batch import
okp batch concepts.ndjson
```

### Invite members

```bash
okp invite create my-domain
okp invite create my-private-domain --role reader
okp invite accept OKP-XXXX-XXXX
okp invite members my-domain
```

### Delete knowledge

```bash
okp delete my-domain/Note/hello --yes
okp domain my-domain --delete --yes
```

## How it works

```
Domain → Concept → Link
```

- **Domain** — a knowledge area with a README and field schema
- **Concept** — one structured knowledge item
- **Link** — relationships between concepts

Access depends on domain visibility:

```
public:  admin/host > writer > reader
private: host > writer > reader
```

Public domains are readable by every authenticated user. Private domains are only discoverable and readable by explicitly invited members, including global admins. Writers can contribute concepts. Each domain has one host, and only the host manages a private domain.

## Read

- [Why OKP is skills-first — and why we chose skills over MCP](https://github.com/talesofai/okp/discussions/1)
- [OKP vs mem0 / claude-mem / basicmemory: knowledge, not memory](https://github.com/talesofai/okp/discussions/2)

## License

MIT
