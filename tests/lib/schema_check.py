#!/usr/bin/env python3
"""tests/schema.sh の検査器（python3 の標準ライブラリだけ）。

JSON Schema（draft 2020-12）のうち、schema/ が使うキーワードだけを実装する。
知らないキーワードが schema にあれば、黙って飛ばさずに落ちる（検査が効いていないのに通るのを防ぐ）。

使い方:
  schema_check.py validate <schema.json> <instance.json>   通れば 0、落ちれば 1 と理由
  schema_check.py examples <contract.md>                    ```json の塊を「節の番号<TAB>JSON（1行）」で出す
  schema_check.py lint <schema.json>                        schema そのものの決まり（下の lint_schema）
"""
import json
import re
import sys

ANNOTATIONS = {"$schema", "$id", "title", "description"}
KEYWORDS = ANNOTATIONS | {
    "$defs", "$ref", "type", "properties", "required", "additionalProperties",
    "items", "prefixItems", "enum", "const", "pattern", "minimum", "minLength",
    "minItems", "oneOf", "anyOf", "allOf", "if", "then", "else", "not",
}
TYPES = {
    "object": lambda v: isinstance(v, dict),
    "array": lambda v: isinstance(v, list),
    "string": lambda v: isinstance(v, str),
    "integer": lambda v: isinstance(v, int) and not isinstance(v, bool),
    "number": lambda v: isinstance(v, (int, float)) and not isinstance(v, bool),
    "boolean": lambda v: isinstance(v, bool),
    "null": lambda v: v is None,
}


class SchemaError(Exception):
    pass


def same(a, b):
    # JSON の等しさ（True と 1 を区別する）
    if isinstance(a, bool) or isinstance(b, bool):
        return type(a) is type(b) and a == b
    return a == b


class Validator:
    def __init__(self, root):
        self.root = root

    def resolve(self, ref):
        if not ref.startswith("#/"):
            raise SchemaError(f"$ref は同じファイルの中だけ: {ref}")
        node = self.root
        for part in ref[2:].split("/"):
            if not isinstance(node, dict) or part not in node:
                raise SchemaError(f"$ref が解けない: {ref}")
            node = node[part]
        return node

    def errors(self, schema, inst, path="$"):
        if schema is True:
            return []
        if schema is False:
            return [f"{path}: 許されていない"]
        if not isinstance(schema, dict):
            raise SchemaError(f"schema が object でない: {schema!r}")
        unknown = set(schema) - KEYWORDS
        if unknown:
            raise SchemaError(f"検査器の知らないキーワード: {sorted(unknown)}")
        errs = []
        if "$ref" in schema:
            errs += self.errors(self.resolve(schema["$ref"]), inst, path)
        if "type" in schema:
            ts = schema["type"] if isinstance(schema["type"], list) else [schema["type"]]
            if not any(TYPES[t](inst) for t in ts):
                return errs + [f"{path}: 型が {'/'.join(ts)} でない"]
        if "enum" in schema and not any(same(inst, e) for e in schema["enum"]):
            errs.append(f"{path}: {json.dumps(inst, ensure_ascii=False)} は {schema['enum']} のどれでもない")
        if "const" in schema and not same(inst, schema["const"]):
            errs.append(f"{path}: {json.dumps(schema['const'])} でない")
        if isinstance(inst, str):
            if "pattern" in schema and not re.search(schema["pattern"], inst):
                errs.append(f"{path}: {json.dumps(inst, ensure_ascii=False)} が形 {schema['pattern']} に合わない")
            if "minLength" in schema and len(inst) < schema["minLength"]:
                errs.append(f"{path}: 短すぎる")
        if TYPES["number"](inst) and "minimum" in schema and inst < schema["minimum"]:
            errs.append(f"{path}: {schema['minimum']} より小さい")
        if isinstance(inst, dict):
            for k in schema.get("required", []):
                if k not in inst:
                    errs.append(f"{path}: キー {k} が無い")
            props = schema.get("properties", {})
            for k, v in inst.items():
                if k in props:
                    errs += self.errors(props[k], v, f"{path}.{k}")
                elif "additionalProperties" in schema:
                    sub = schema["additionalProperties"]
                    if sub is False:
                        errs.append(f"{path}: 知らないキー {k}")
                    else:
                        errs += self.errors(sub, v, f"{path}.{k}")
        if isinstance(inst, list):
            if "minItems" in schema and len(inst) < schema["minItems"]:
                errs.append(f"{path}: 要素が {schema['minItems']} より少ない")
            prefix = schema.get("prefixItems", [])
            for i, v in enumerate(inst):
                if i < len(prefix):
                    errs += self.errors(prefix[i], v, f"{path}[{i}]")
                elif "items" in schema:
                    errs += self.errors(schema["items"], v, f"{path}[{i}]")
        for sub in schema.get("allOf", []):
            errs += self.errors(sub, inst, path)
        for k in ("anyOf", "oneOf"):
            if k not in schema:
                continue
            branches = [self.errors(s, inst, path) for s in schema[k]]
            n = sum(1 for b in branches if not b)
            if n == 0:
                # 一番近い枝（理由が最も少ない枝）の理由を添える
                nearest = min(branches, key=len)
                errs.append(f"{path}: {k} のどれにも合わない（近い枝: {nearest[0]}）")
            elif k == "oneOf" and n > 1:
                errs.append(f"{path}: oneOf のうち {n} 個に合う（ちょうど1つでない）")
        if "not" in schema and not self.errors(schema["not"], inst, path):
            errs.append(f"{path}: not の形に合ってしまう")
        if "if" in schema:
            branch = "then" if not self.errors(schema["if"], inst, path) else "else"
            if branch in schema:
                errs += self.errors(schema[branch], inst, path)
        return errs


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


def main(argv):
    if len(argv) >= 2 and argv[1] == "validate" and len(argv) == 4:
        schema = load(argv[2])
        inst = load(argv[3])
        errs = Validator(schema).errors(schema, inst)
        for e in errs:
            print(e)
        return 1 if errs else 0
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
    except (SchemaError, json.JSONDecodeError, OSError) as e:
        print(f"検査できない: {e}", file=sys.stderr)
        sys.exit(2)
