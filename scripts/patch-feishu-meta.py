#!/usr/bin/env python3
"""patch-feishu-meta.py — 从 silo server 补全 feishu-social 的 frontmatter

从 kjx-personal silo server 读取 feishu-* collections 的完整 meta，
将 sender/date/platform/url/group/stars/forks/likes/views/time 等字段
写入 okp 对应 concept 的 frontmatter。

用法:
  OKP_API_TOKEN=<token> python3 patch-feishu-meta.py [--dry-run]
"""

import json
import os
import re
import sys
import time
import urllib.request
import argparse

SILO_BASE = "https://s-98d87d78-047f-4298-9b7e-ea12ef39f0ae-5173.cohub.run"
OKP_BASE = os.environ.get("OKP_API_BASE", "https://okp.neta.art")
OKP_TOKEN = os.environ.get("OKP_API_TOKEN", "")

FEISHU_COLLECTIONS = [
    "feishu-100x",
    "feishu-blogger",
    "feishu-comfy",
    "feishu-pov",
    "feishu-worldbuild",
    "feishu-spacebuild",
]

# meta 字段白名单（写入 frontmatter）
META_FIELDS = [
    "sender", "date", "platform", "url", "time", "group",
    "stars", "forks", "likes", "views", "collects", "shares", "comments",
    "topic", "scenario", "content_type", "source_platform",
]


def silo_request(path):
    req = urllib.request.Request(f"{SILO_BASE}{path}")
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.loads(resp.read())


def okp_request(method, path, body=None):
    url = f"{OKP_BASE}{path}"
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    req.add_header("Authorization", f"Bearer {OKP_TOKEN}")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            return json.loads(resp.read()), resp.status
    except urllib.error.HTTPError as e:
        return json.loads(e.read()), e.code


def slugify(title):
    s = title.strip().lower()
    s = re.sub(r'[^\w\s-]', '', s)
    s = re.sub(r'[-\s]+', '-', s)
    return s[:80]


def fetch_all_silo_items(collection_id):
    """分页拉取 silo collection 的所有 items（含 meta）"""
    items = []
    cursor = None
    while True:
        path = f"/api/items?collection={collection_id}&limit=200"
        if cursor:
            path += f"&after={cursor}"
        data = silo_request(path)
        batch = data.get("items", [])
        if not batch:
            break
        items.extend(batch)
        if not data.get("has_more"):
            break
        cursor = data.get("next_cursor")
    return items


def patch_collection(collection_id, dry_run=False):
    print(f"\n▶ {collection_id}")
    items = fetch_all_silo_items(collection_id)
    print(f"  silo items: {len(items)}")

    updated = skipped = errors = 0

    for item in items:
        title = item.get("title") or item.get("filename", "").replace(".md", "")
        if not title:
            continue

        slug = slugify(title)
        concept_id = f"feishu-social/Link/{slug}"

        # 读取 okp 已有 concept
        existing, status = okp_request("GET", f"/api/v1/concepts/{concept_id}")
        if status != 200:
            skipped += 1
            continue  # concept 不存在，跳过

        # 构建新 frontmatter：保留原有字段 + 补入 meta 字段
        fm = dict(existing.get("frontmatter") or {})
        meta = item.get("meta") or {}
        if isinstance(meta, str):
            try:
                meta = json.loads(meta)
            except Exception:
                meta = {}

        changed = False
        for field in META_FIELDS:
            if field in meta and meta[field] not in (None, "", []):
                if fm.get(field) != meta[field]:
                    fm[field] = meta[field]
                    changed = True

        # source_url → url
        if item.get("source_url") and not fm.get("url"):
            fm["url"] = item["source_url"]
            changed = True

        if not changed:
            skipped += 1
            continue

        if dry_run:
            print(f"  [DRY] {concept_id}: would update fm={list(fm.keys())}")
            updated += 1
            continue

        # PUT 更新 concept（保留其他字段不变）
        payload = dict(existing)
        payload["frontmatter"] = fm
        _, put_status = okp_request("PUT", f"/api/v1/concepts/{concept_id}", payload)
        if put_status == 200:
            updated += 1
        else:
            errors += 1
            if errors <= 3:
                print(f"  ❌ {concept_id}: PUT {put_status}")

        time.sleep(0.05)

    status_icon = "🔍" if dry_run else ("✅" if errors == 0 else "⚠️")
    print(f"  {status_icon} updated={updated} skipped={skipped} errors={errors}")
    return updated, skipped, errors


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    if not OKP_TOKEN:
        print("❌ OKP_API_TOKEN 未设置", file=sys.stderr)
        sys.exit(1)

    # 验证 okp 连通性
    _, status = okp_request("GET", "/api/v1/health")
    if status != 200:
        print("❌ okp API 不可达", file=sys.stderr)
        sys.exit(1)
    print(f"✅ okp API: {OKP_BASE}")
    print(f"{'[DRY-RUN] ' if args.dry_run else ''}开始补全 feishu-social frontmatter...")

    total_updated = total_skipped = total_errors = 0
    for cid in FEISHU_COLLECTIONS:
        u, s, e = patch_collection(cid, dry_run=args.dry_run)
        total_updated += u
        total_skipped += s
        total_errors += e

    print(f"\n总计: updated={total_updated} skipped={total_skipped} errors={total_errors}")


if __name__ == "__main__":
    main()
