#!/usr/bin/env python3
"""schema そのものの決まりの検査（lint）と、文書の例の取り出し（python3 の標準ライブラリだけ）。tests/schema.sh・tests/process.sh が使う。

JSON を schema と照らす検査器ではない。照合は andon schema check（command が約束の JSON を書く前に照らすのと同じ、
internal/contract の検査器）でする（ADR-0034 決めたこと 6）。

使い方:
  schema_check.py examples <contract.md>   ```json の塊を「節の番号<TAB>JSON（1行）」で出す
  schema_check.py lint <schema.json>       schema そのものの決まり（下の lint_schema）。通れば 0、落ちれば 1 と理由
  schema_check.py batch <cases.tsv>        lint を1行1件でまとめて走らせる（python の起動を1回にする）
                                           行: `lint<TAB>schema`。出力: 行ごとに `終了コード<TAB>理由の1行目`（lint 単体と同じ終了コード）
"""
import json
import re
import sys

# 検査器（internal/contract の Keywords）が読むキーワード。同じ顔ぶれにする。
# 標準に無いキーワードは1つだけ: `uniqueKeys`（配列の object の要素で、名前を並べたキーの値が重ならない。例 候補の `id`）。
ANNOTATIONS = {"$schema", "$id", "title", "description"}
KEYWORDS = ANNOTATIONS | {
    "$defs", "$ref", "type", "properties", "required", "additionalProperties",
    "items", "prefixItems", "enum", "const", "pattern", "minimum", "minLength",
    "minItems", "oneOf", "anyOf", "allOf", "if", "then", "else", "not",
    "uniqueKeys",
}


def divergent(pattern):
    """ECMA-262（JSON Schema の pattern の方言）と Go の regexp（andon の検査器）で意味が割れるか、Go で読めない表現を返す
    （schema/ の pattern には使わない）。

    意味が割れる: `.`（行区切り文字 U+2028 などの扱い）と、`\\s` `\\d` `\\w` `\\b` とその大文字（Unicode の範囲）。
    代わりに明示した ASCII の文字クラスで書く。`$`（multiline なし）はどちらも入力の末尾だけに合うので使ってよい。
    Go で読めない: 先読み・後読み（`(?=` `(?!` `(?<=` `(?<!`）、後方参照（`\\1` など）、`\\u` の逃がし。
    文字は `\\x00` の形で書き、先読みは段ごとの選択肢に書き直す。
    """
    found, i, in_class = [], 0, False
    while i < len(pattern):
        c = pattern[i]
        if c == "\\":
            nxt = pattern[i + 1:i + 2]
            if nxt in set("sSdDwWbBu123456789"):
                found.append(pattern[i:i + 2])
            i += 2
            continue
        if in_class:
            in_class = c != "]"
        elif c == "[":
            in_class = True
        elif c == ".":
            found.append(".")
        elif c == "(":
            for look in ("(?=", "(?!", "(?<=", "(?<!"):
                if pattern.startswith(look, i):
                    found.append(look)
                    break
        i += 1
    return found


def load(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def walk(schema, at="$"):
    """schema の中の部分 schema を全部たどる（lint 用）。"""
    yield at, schema
    if not isinstance(schema, dict):
        return
    for k in ("properties", "$defs"):
        for name, sub in schema.get(k, {}).items():
            yield from walk(sub, f"{at}.{k}.{name}")
    for k in ("additionalProperties", "items", "not", "if", "then", "else"):
        if isinstance(schema.get(k), dict):
            yield from walk(schema[k], f"{at}.{k}")
    for k in ("prefixItems", "oneOf", "anyOf", "allOf"):
        for i, sub in enumerate(schema.get(k, [])):
            yield from walk(sub, f"{at}.{k}[{i}]")


def lint_schema(schema):
    """schema の決まり: 2020-12・$id に版・description と識別子は英語（ASCII）・プロパティには description。"""
    errs = []
    if schema.get("$schema") != "https://json-schema.org/draft/2020-12/schema":
        errs.append("$schema が draft 2020-12 でない")
    if not re.fullmatch(r"https://andon\.example\.com/schema/[a-z-]+/[1-9][0-9]*", schema.get("$id", "")):
        errs.append("$id が https://andon.example.com/schema/<name>/<版> でない")
    for at, sub in walk(schema):
        if not isinstance(sub, dict):
            continue
        for k in sub:
            if k not in KEYWORDS:
                errs.append(f"{at}: 検査器の知らないキーワード {k}")
        for k in ("title", "description"):
            if k in sub and not sub[k].isascii():
                errs.append(f"{at}.{k}: 英語でない（ASCII の外の字がある）")
        if isinstance(sub.get("pattern"), str):
            for d in divergent(sub["pattern"]):
                errs.append(f"{at}.pattern: ECMA-262 と Go で意味が割れるか、Go で読めない {d} を使っている（divergent の説明の形で書く）")
        for name, prop in sub.get("properties", {}).items():
            if not name.isascii():
                errs.append(f"{at}: 識別子 {name} が英語でない")
            # if/then/else の中は条件の写しなので、元の宣言の description で足りる
            conditional = re.search(r"\.(if|then|else)\b", at)
            if isinstance(prop, dict) and "description" not in prop and not conditional:
                errs.append(f"{at}.properties.{name}: description が無い")
    if "description" not in schema:
        errs.append("根に description が無い")
    return errs


def examples(md_path):
    """contract.md の ```json の塊を、属する節の番号（## N.）と組にして出す。"""
    section, block, out = None, None, []
    with open(md_path, encoding="utf-8") as f:
        for line in f:
            m = re.match(r"## (\d+)\. ", line)
            if block is None and m:
                section = m.group(1)
            if block is None and line.strip() == "```json":
                block = []
            elif block is not None and line.strip() == "```":
                out.append((section, json.loads("".join(block))))
                block = None
            elif block is not None:
                block.append(line)
    for sec, doc in out:
        print(f"{sec}\t{json.dumps(doc, ensure_ascii=False)}")


def run_case(fields):
    """batch の1件。(終了コード, 理由の1行目)。単体の lint と同じ終了コードを返す。"""
    try:
        if fields[0] == "lint" and len(fields) == 2:
            errs = lint_schema(load(fields[1]))
        else:
            return 2, "検査できない: batch の行の形が違う（照合は andon schema batch）"
    except (json.JSONDecodeError, OSError) as e:
        return 2, f"検査できない: {e}"
    return (1, errs[0]) if errs else (0, "")


def main(argv):
    if len(argv) == 3 and argv[1] == "batch":
        with open(argv[2], encoding="utf-8") as f:
            for line in f.read().split("\n"):
                if line == "":
                    continue
                rc, why = run_case(line.split("\t"))
                print(f"{rc}\t{why}".replace("\n", " "))
        return 0
    if len(argv) == 3 and argv[1] == "examples":
        examples(argv[2])
        return 0
    if len(argv) == 3 and argv[1] == "lint":
        errs = lint_schema(load(argv[2]))
        for e in errs:
            print(e)
        return 1 if errs else 0
    print(__doc__, file=sys.stderr)
    return 2


if __name__ == "__main__":
    try:
        sys.exit(main(sys.argv))
    except (json.JSONDecodeError, OSError) as e:
        print(f"検査できない: {e}", file=sys.stderr)
        sys.exit(2)
