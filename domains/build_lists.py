#!/usr/bin/env python3
"""Сборка доменных списков для amnezia-box / sing-box.

Вход:  сырые списки источников + локальные include/exclude.
Выход: vpn-domains.lst  — чистый список, по домену на строку
       vpn-domains.json — source rule-set для `sing-box rule-set compile`

Нормализация: lowercase, punycode (IDN), срезание *. / протоколов / путей,
выброс IP-адресов и мусора, дедупликация, удаление доменов, покрытых
родительским суффиксом (www.example.com при наличии example.com не нужен —
domain_suffix и так его матчит).
"""

import argparse
import ipaddress
import json
import re
import sys

LABEL = r"[a-z0-9_]([a-z0-9_-]{0,61}[a-z0-9_])?"
DOMAIN_RE = re.compile(rf"^({LABEL}\.)+{LABEL}$", re.IGNORECASE)


def normalize(raw: str):
    """Одна строка -> домен или None."""
    line = raw.strip().lower()
    if not line or line.startswith("#"):
        return None
    # hosts-формат: "0.0.0.0 example.com"
    parts = line.split()
    if len(parts) > 1:
        try:
            ipaddress.ip_address(parts[0])
            line = parts[1]
        except ValueError:
            line = parts[0]
    else:
        line = parts[0]
    line = line.split("#", 1)[0].strip()
    # url-формат
    if "://" in line:
        line = line.split("://", 1)[1]
    line = line.split("/", 1)[0]
    line = line.split(":", 1)[0]  # порт
    # wildcard / leading dot / trailing dot
    line = line.lstrip("*").lstrip(".").rstrip(".")
    if not line:
        return None
    # IDN -> punycode
    try:
        line = line.encode("idna").decode("ascii")
    except (UnicodeError, UnicodeDecodeError):
        return None
    # IP-адреса не нужны — это доменный список
    try:
        ipaddress.ip_address(line)
        return None
    except ValueError:
        pass
    return line


def load_files(paths):
    """Читает файлы, возвращает (set доменов, статистика)."""
    domains = set()
    stats = {"raw": 0, "dropped": 0, "tld": []}
    for path in paths:
        try:
            with open(path, encoding="utf-8", errors="replace") as f:
                for raw in f:
                    stats["raw"] += 1
                    d = normalize(raw)
                    if d is None:
                        if raw.strip() and not raw.strip().startswith("#"):
                            stats["dropped"] += 1
                        continue
                    if "." not in d:
                        stats["tld"].append(d)
                    domains.add(d)
        except FileNotFoundError:
            print(f"WARN: файл не найден: {path}", file=sys.stderr)
    return domains, stats


def suffix_exclude(domains: set, exclude: set) -> set:
    """Выкидывает домены, равные exclude-записи или являющиеся их поддоменами."""
    if not exclude:
        return domains
    return {
        d
        for d in domains
        if not any(d == e or d.endswith("." + e) for e in exclude)
    }


def suffix_dedup(domains: set) -> list:
    """Убирает домены, покрытые другой записью-суффиксом."""
    keep = []
    for d in sorted(domains):
        labels = d.split(".")
        covered = any(".".join(labels[i:]) in domains for i in range(1, len(labels)))
        if not covered:
            keep.append(d)
    return keep


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("sources", nargs="+", help="сырые списки доменов (vpn)")
    ap.add_argument("--exclude-files", nargs="*", default=[], help="списки force-direct")
    ap.add_argument("--out-lst", required=True)
    ap.add_argument("--out-json", required=True)
    args = ap.parse_args()

    vpn, st1 = load_files(args.sources)
    excl, st2 = load_files(args.exclude_files)

    before = len(vpn)
    vpn = suffix_exclude(vpn, excl)
    excluded = before - len(vpn)

    final = suffix_dedup(vpn)
    covered = len(vpn) - len(final)

    with open(args.out_lst, "w", encoding="utf-8") as f:
        f.write("\n".join(final) + "\n")

    ruleset = {"version": 3, "rules": [{"domain_suffix": final}]}
    with open(args.out_json, "w", encoding="utf-8") as f:
        json.dump(ruleset, f, ensure_ascii=False, indent=1)
        f.write("\n")

    print(f"vpn-источники:      {st1['raw']} строк -> {len(vpn) + excluded + 0} доменов")
    print(f"выброшено мусора:   {st1['dropped']}")
    print(f"exclude (direct):   {st2['raw']} строк -> {len(excl)} записей, вычтено {excluded}")
    print(f"покрыто суффиксом:  {covered}")
    print(f"ИТОГО в списке:     {len(final)}")
    tlds = sorted(set(st1["tld"]))
    if tlds:
        print(f"ВНИМАНИЕ, TLD-записи (матчат целую зону): {', '.join(tlds)}", file=sys.stderr)


if __name__ == "__main__":
    main()
