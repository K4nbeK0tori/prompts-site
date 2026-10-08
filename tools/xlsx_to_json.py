#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""把「焚绝预设合集」xlsx 转成 prompts-server 可导入的 JSON。

用法:
    python tools/xlsx_to_json.py "焚绝预设合集_整理 (1).xlsx" data/prompts.json

依赖: openpyxl  (pip install openpyxl)
"""
import json
import re
import sys
from datetime import datetime
from pathlib import Path

import openpyxl

# xlsx 表头 -> 输出字段
COL_TITLE = "名称"
COL_TAGS = "标签"
COL_SOURCE = "来源"
COL_ENABLED = "启用"
COL_CONTENT = "提示词"
COL_TIME = "更新时间"
COL_INDEX = "序号"

TAG_SPLIT = re.compile(r"[、,，;；|/\s]+")


def norm_text(value) -> str:
    if value is None:
        return ""
    text = str(value)
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    return text.strip()


def to_rfc3339(value) -> str:
    """把 '2026-03-05 01:28:45.356 +0800' 之类的写法统一成 RFC3339。"""
    if value is None:
        return ""
    if isinstance(value, datetime):
        return value.strftime("%Y-%m-%dT%H:%M:%S")
    text = str(value).strip()
    if not text:
        return ""
    patterns = (
        "%Y-%m-%d %H:%M:%S.%f %z",
        "%Y-%m-%d %H:%M:%S %z",
        "%Y-%m-%d %H:%M:%S",
        "%Y-%m-%d %H:%M",
        "%Y-%m-%d",
    )
    for fmt in patterns:
        try:
            dt = datetime.strptime(text, fmt)
        except ValueError:
            continue
        return dt.isoformat(timespec="seconds")
    return ""


def split_tags(raw) -> list:
    text = norm_text(raw)
    if not text:
        return []
    seen, out = set(), []
    for piece in TAG_SPLIT.split(text):
        piece = piece.strip().lstrip("#").strip()
        if not piece:
            continue
        key = piece.lower()
        if key in seen:
            continue
        seen.add(key)
        out.append(piece)
    return out


def truthy(raw, default=True) -> bool:
    text = norm_text(raw).lower()
    if text == "":
        return default
    return text not in {"否", "no", "false", "0", "off", "停用", "禁用"}


def derive_title(content: str) -> str:
    line = content.split("\n", 1)[0].strip().strip("#").strip()
    if not line:
        return "未命名提示词"
    return line[:24] + "…" if len(line) > 24 else line


def convert(xlsx_path: Path) -> list:
    wb = openpyxl.load_workbook(xlsx_path, data_only=True)
    ws = wb.worksheets[0]
    rows = list(ws.iter_rows(values_only=True))
    if not rows:
        return []

    header = [norm_text(c) for c in rows[0]]
    idx = {name: header.index(name) for name in
           (COL_INDEX, COL_TITLE, COL_TAGS, COL_SOURCE, COL_ENABLED, COL_CONTENT, COL_TIME)
           if name in header}
    missing = [n for n in (COL_TITLE, COL_CONTENT) if n not in idx]
    if missing:
        raise SystemExit(f"表头缺少必需列: {missing}，实际表头: {header}")

    out = []
    for row in rows[1:]:
        def cell(name):
            pos = idx.get(name)
            return row[pos] if pos is not None and pos < len(row) else None

        content = norm_text(cell(COL_CONTENT))
        if not content:
            continue

        title = norm_text(cell(COL_TITLE)) or derive_title(content)
        stamp = to_rfc3339(cell(COL_TIME))
        try:
            order = int(str(cell(COL_INDEX)).strip())
        except (TypeError, ValueError):
            order = None

        record = {
            "title": title,
            "content": content,
            "tags": split_tags(cell(COL_TAGS)),
            "source": norm_text(cell(COL_SOURCE)),
            "enabled": truthy(cell(COL_ENABLED), default=True),
            "sort_order": order,
        }
        if stamp:
            record["created_at"] = stamp
            record["updated_at"] = stamp
        out.append(record)
    return out


def main() -> None:
    if len(sys.argv) < 2:
        print(__doc__)
        raise SystemExit(2)
    src = Path(sys.argv[1])
    dst = Path(sys.argv[2]) if len(sys.argv) > 2 else Path("data/prompts.json")
    if not src.exists():
        raise SystemExit(f"找不到文件: {src}")

    records = convert(src)
    dst.parent.mkdir(parents=True, exist_ok=True)
    dst.write_text(json.dumps(records, ensure_ascii=False, indent=1), encoding="utf-8")

    tags = {}
    for r in records:
        for t in r["tags"]:
            tags[t] = tags.get(t, 0) + 1
    print(f"已写入 {len(records)} 条 -> {dst}")
    print(f"标签 {len(tags)} 个: " + ", ".join(
        f"{k}({v})" for k, v in sorted(tags.items(), key=lambda kv: -kv[1])))


if __name__ == "__main__":
    main()
