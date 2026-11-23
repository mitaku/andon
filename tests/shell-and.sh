#!/usr/bin/env bash
# tests/shell-and.sh [<ファイル> …]
#
# set -e は A && B の A が偽でも止まらない。tests/ の下のシェルで、判定を A && B と並べたのに失敗の受け手（|| die など）が無い
# 行を探し、ファイルと行を出して落ちる。拾うのは次の全部に当たる and-or の並び:
#   - && でつなぎ、|| を含まない（|| があれば受け手がある）。& で裏に回していない。
#   - 最後の項が判定（[・[[・test・((・grep・cmp・diff・jq -e か、! で反転した項）。最後が mv・echo・die・{ … } なら、
#     条件つきの動作か「A が真なら落とす」逆の判定で、A が偽で進むのが正しい（[ -n "$x" ] && echo … は拾わない）。
#   - set -e が効く所にある。if・elif・while・until の条件、関数とサブシェルの本体の最後（偽が外へ返る）、
#     && と || の最後でない項と ! の中（( … ) || die のサブシェルの中など。set -e が中身ごと効かない）、
#     $( … ) の中と、set -e を書いていないファイル（pass と fail で判定を受ける）は見ない。
# 引数なしは、自分の読み手を見本（拾う形と拾わない形）で確かめてから、tests/ の下のシェルを全部見る。見るのは、git が追跡するか
# まだ add していない新しいファイルのうち、*.sh と先頭行が bash か sh の #! のもの。引数を渡すと、そのファイルだけを見る
# （見本の確かめは飛ばす）。
# 構文の読み手は python3 の標準ライブラリだけで書いた（here-doc・引用符・$( )・[[ ]]・case を読む）。読めない形に当たったら落ちる。
set -euo pipefail
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

scan() { # scan <ファイル> …: 拾った行を「<ファイル>:<行>: <並び>」で出す。読めないファイルがあれば終了コード 2
  python3 - "$@" <<'PY'
import bisect, re, sys

META = set(" \t\n;&|()<>")
# ) は case のパターンの閉じ（a) [[ … ]] …）。サブシェルの ) の直後にコマンドは来ない
CMD_PREV_OPS = {"&&", "||", ";", "&", "|", "|&", "(", ")", ";;", ";&", ";;&"}
CMD_PREV_WORDS = {"then", "do", "else", "elif", "if", "while", "until", "!", "{", "time", "for"}
CHECKS = {"[", "test", "grep", "egrep", "fgrep", "cmp", "diff"}
# 成功を言うだけの項（A && ok "…"）。A が偽なら言わずに先へ進むので、判定の続きとして扱う
REPORTS = {"ok", "pass"}
# 受け手にならないもの。|| echo "FAIL …" >&2 は言うだけで止めない。ほかの受け手（die・fail・ng・exit・true など。
# 検査ごとに定義した関数を含む）は、止めるか失敗を数えるか、失敗を流すと書いたものとして通す
TALKS = {"echo", "printf"}
# 前に置く道具（timeout 5 grep …・env X=1 grep …・command grep …）。外してから名前を見る。
# 値は（名前の後ろの位置の引数の数, 値を取る旗）
WRAPPERS = {
    "command": (0, ()),
    "env": (0, ("-u", "-C", "-S")),
    "timeout": (1, ("-s", "-k")),
    "nice": (0, ("-n",)),
    "stdbuf": (0, ("-i", "-o", "-e")),
}


class ReadError(Exception):
    pass


class Tok:
    def __init__(self, kind, val, pos, target=False):
        self.kind, self.val, self.pos, self.target = kind, val, pos, target


class Lexer:
    def __init__(self, src):
        self.s = src
        self.n = len(src)

    def err(self, i, msg):
        raise ReadError((i, msg))

    # 引用符・$( )・${ }・` ` を1つ読み飛ばす。i は開きの位置、返すのは閉じの次の位置
    def single(self, i):
        j = self.s.find("'", i + 1)
        if j < 0:
            self.err(i, "' が閉じていない")
        return j + 1

    def ansi(self, i):  # $'...'
        j = i + 2
        while j < self.n:
            if self.s[j] == "\\":
                j += 2
                continue
            if self.s[j] == "'":
                return j + 1
            j += 1
        self.err(i, "$' が閉じていない")

    def double(self, i):
        j = i + 1
        while j < self.n:
            c = self.s[j]
            if c == "\\":
                j += 2
            elif c == '"':
                return j + 1
            elif c == "$" and self.s.startswith("$(", j):
                j = self.dollar_paren(j)
            elif c == "$" and self.s.startswith("${", j):
                j = self.brace(j)
            elif c == "`":
                j = self.backtick(j)
            else:
                j += 1
        self.err(i, '" が閉じていない')

    def backtick(self, i):
        j = i + 1
        while j < self.n:
            if self.s[j] == "\\":
                j += 2
                continue
            if self.s[j] == "`":
                return j + 1
            j += 1
        self.err(i, "` が閉じていない")

    def brace(self, i):  # ${ ... }
        j, depth = i + 2, 1
        while j < self.n:
            c = self.s[j]
            if c == "\\":
                j += 2
            elif c == "'":
                j = self.single(j)
            elif c == '"':
                j = self.double(j)
            elif c == "$" and self.s.startswith("$(", j):
                j = self.dollar_paren(j)
            elif c == "$" and self.s.startswith("${", j):
                j = self.brace(j)
            elif c == "`":
                j = self.backtick(j)
            elif c == "}":
                depth -= 1
                j += 1
                if depth == 0:
                    return j
            else:
                j += 1
        self.err(i, "${ が閉じていない")

    def arith(self, i, open_len):  # $(( ... )) か (( ... ))。括弧を数える
        j, depth = i + open_len, 2
        while j < self.n:
            c = self.s[j]
            if c == "(":
                depth += 1
            elif c == ")":
                depth -= 1
                if depth == 0:
                    return j + 1
            elif c == "'":
                j = self.single(j) - 1
            elif c == '"':
                j = self.double(j) - 1
            elif c == "$" and self.s.startswith("${", j):
                j = self.brace(j) - 1
            j += 1
        self.err(i, "(( が閉じていない")

    def dollar_paren(self, i):  # $( ... ) か $(( ... ))
        if self.s.startswith("$((", i):
            # $(( は算術。ただし $( (cd x) ) のような書き方は bash も算術と読むので同じに扱う
            return self.arith(i, 3)
        _, j = self.tokens(i + 2, nested=True)
        return j

    def paren_group(self, i):  # <( ... )・>( ... )・a=( ... )・@( ... ) の ( から
        _, j = self.tokens(i + 1, nested=True)
        return j

    def word(self, i):
        """語を1つ読む。返すのは (語の文字列, 次の位置)。"""
        j = i
        while j < self.n:
            c = self.s[j]
            if c == "\\":
                if j + 1 < self.n and self.s[j + 1] == "\n":
                    j += 2
                    continue
                j += 2
            elif c == "'":
                j = self.single(j)
            elif c == '"':
                j = self.double(j)
            elif c == "$" and self.s.startswith("$'", j):
                j = self.ansi(j)
            elif c == "$" and self.s.startswith("$(", j):
                j = self.dollar_paren(j)
            elif c == "$" and self.s.startswith("${", j):
                j = self.brace(j)
            elif c == "`":
                j = self.backtick(j)
            elif c == "(" and j > i and (self.s[j - 1] == "=" or self.s[j - 1] in "@*+?!"):
                j = self.paren_group(j)  # 配列 a=( … ) と extglob
            elif c in META:
                break
            else:
                j += 1
        return self.s[i:j], j

    def heredoc_bodies(self, i, pending):
        """改行の次の位置 i から、溜まった here-doc の本文を読み飛ばす。"""
        for delim, strip in pending:
            while True:
                if i >= self.n:
                    return i  # 本文が閉じないまま終わった。bash は警告して読む
                e = self.s.find("\n", i)
                line = self.s[i:] if e < 0 else self.s[i:e]
                i = self.n if e < 0 else e + 1
                if (line.lstrip("\t") if strip else line) == delim:
                    break
        pending.clear()
        return i

    def at_cmd(self, toks):
        if not toks:
            return True
        p = toks[-1]
        if p.kind == "NL":
            return True
        if p.kind == "OP":
            return p.val in CMD_PREV_OPS
        return p.kind == "WORD" and p.val in CMD_PREV_WORDS

    def tokens(self, i, nested=False):
        """語と演算子の列を作る。nested は $( … ) などの中で、対になる ) で返る。"""
        toks, pending, depth = [], [], 0
        s, n = self.s, self.n
        while i < n:
            c = s[i]
            if c in " \t":
                i += 1
                continue
            if c == "\\" and s.startswith("\\\n", i):
                i += 2
                continue
            if c == "\n":
                toks.append(Tok("NL", "\n", i))
                i = self.heredoc_bodies(i + 1, pending) if pending else i + 1
                continue
            if c == "#":
                e = s.find("\n", i)
                i = n if e < 0 else e
                continue
            if s.startswith("((", i) and self.at_cmd(toks):
                j = self.arith(i, 2)
                toks.append(Tok("ARITH", s[i:j], i))
                i = j
                continue
            if s.startswith("<(", i) or s.startswith(">(", i):
                j = self.paren_group(i + 1)
                toks.append(Tok("WORD", s[i:j], i))
                i = j
                continue
            if c in "<>" or s.startswith("&>", i):
                for op in ("<<<", "<<-", "<<", "&>>", "&>", "<>", "<&", ">&", ">>", ">|", "<", ">"):
                    if s.startswith(op, i):
                        break
                i += len(op)
                if op in ("<<", "<<-"):
                    while i < n and s[i] in " \t":
                        i += 1
                    w, i = self.word(i)
                    if not w:
                        self.err(i, "here-doc の区切りが無い")
                    pending.append((re.sub(r"[\"'\\]", "", w), op == "<<-"))
                    toks.append(Tok("REDIR", op, i))
                else:
                    toks.append(Tok("REDIR", op, i, target=True))
                continue
            if c in ";&|()":
                for op in (";;&", ";;", ";&", "&&", "||", "|&", ";", "&", "|", "(", ")"):
                    if s.startswith(op, i):
                        break
                if op == "(":
                    depth += 1
                elif op == ")":
                    if nested and depth == 0:
                        return toks, i + 1
                    depth -= 1
                toks.append(Tok("OP", op, i))
                i += len(op)
                continue
            start = i
            w, i = self.word(i)
            if w == "[[" and self.at_cmd(toks):
                i = self.dbrack(i)
                toks.append(Tok("TEST", s[start:i], start))
                continue
            if w.isdigit() and i < n and s[i] in "<>":
                continue  # 2>&1 の 2 は向け先の番号
            toks.append(Tok("WORD", w, start))
        if nested:
            self.err(i, "$( が閉じていない")
        return toks, i

    def dbrack(self, i):
        """[[ の次から ]] の次まで。中の && や ( は演算子として扱わない。"""
        s, n = self.s, self.n
        while i < n:
            if s[i] in " \t\n":
                i += 1
            elif s.startswith("\\\n", i):
                i += 2
            elif s.startswith("]]", i) and (i + 2 >= n or s[i + 2] in META):
                return i + 2
            elif s[i] in "&|<>()!;":
                i += 1
            else:
                _, j = self.word(i)
                i = j if j > i else i + 1
        self.err(i, "[[ が閉じていない")


class Node:
    def __init__(self, **kw):
        self.__dict__.update(kw)


class Parser:
    def __init__(self, toks):
        self.t, self.i = toks, 0

    def peek(self, k=0):
        j = self.i + k
        return self.t[j] if j < len(self.t) else None

    def take(self):
        t = self.peek()
        self.i += 1
        return t

    def err(self, msg):
        t = self.peek()
        raise ReadError((t.pos if t else -1, msg))

    def skip_nl(self):
        while self.peek() and self.peek().kind == "NL":
            self.i += 1

    def is_word(self, val, k=0):
        t = self.peek(k)
        return t is not None and t.kind == "WORD" and t.val == val

    def expect_word(self, val):
        if not self.is_word(val):
            self.err(f"{val} が無い")
        self.i += 1

    def expect_op(self, val):
        t = self.peek()
        if t is None or t.kind != "OP" or t.val != val:
            self.err(f"{val} が無い")
        self.i += 1

    def stops(self, words, ops):
        t = self.peek()
        if t is None:
            return True
        if t.kind == "WORD" and t.val in words:
            return True
        return t.kind == "OP" and t.val in ops

    def parse_list(self, words=(), ops=()):
        stmts = []
        while True:
            while self.peek() and (self.peek().kind == "NL" or (self.peek().kind == "OP" and self.peek().val == ";")):
                self.i += 1
            if self.stops(words, ops):
                return stmts
            st = self.andor()
            t = self.peek()
            st.bg = False
            if t is not None and t.kind == "OP" and t.val in (";", "&"):
                st.bg = t.val == "&"
                self.i += 1
            elif t is not None and t.kind == "NL":
                self.i += 1
            elif not self.stops(words, ops):
                self.err("並びの後ろが読めない")
            stmts.append(st)

    def andor(self):
        items, ops = [self.pipeline()], []
        while self.peek() and self.peek().kind == "OP" and self.peek().val in ("&&", "||"):
            ops.append(self.take().val)
            self.skip_nl()
            items.append(self.pipeline())
        return Node(items=items, ops=ops, pos=items[0].pos)

    def pipeline(self):
        pos = self.peek().pos if self.peek() else -1
        neg = False
        while self.is_word("!") or self.is_word("time"):
            neg = neg or self.take().val == "!"
        cmds = [self.command()]
        while self.peek() and self.peek().kind == "OP" and self.peek().val in ("|", "|&"):
            self.i += 1
            self.skip_nl()
            cmds.append(self.command())
        return Node(neg=neg, cmds=cmds, pos=pos)

    def redirs(self):
        while self.peek() and self.peek().kind == "REDIR":
            if self.take().target:
                self.take()

    def command(self):
        t = self.peek()
        if t is None:
            self.err("コマンドが無い")
        if t.kind == "OP" and t.val == "(":
            self.i += 1
            body = self.parse_list(ops=(")",))
            self.expect_op(")")
            self.redirs()
            return Node(kind="subshell", body=body)
        if t.kind in ("TEST", "ARITH"):
            self.i += 1
            self.redirs()
            return Node(kind="test")
        if t.kind == "WORD":
            v = t.val
            if v == "{":
                self.i += 1
                body = self.parse_list(words=("}",))
                self.expect_word("}")
                self.redirs()
                return Node(kind="group", body=body)
            if v == "if":
                self.i += 1
                bodies = []
                while True:
                    self.parse_list(words=("then",))  # 条件は見ない
                    self.expect_word("then")
                    bodies.append(self.parse_list(words=("elif", "else", "fi")))
                    if self.is_word("elif"):
                        self.i += 1
                        continue
                    if self.is_word("else"):
                        self.i += 1
                        bodies.append(self.parse_list(words=("fi",)))
                    self.expect_word("fi")
                    break
                self.redirs()
                return Node(kind="if", bodies=bodies)
            if v in ("while", "until"):
                self.i += 1
                self.parse_list(words=("do",))  # 条件は見ない
                return self.loop_body()
            if v in ("for", "select"):
                self.i += 1
                while True:
                    u = self.peek()
                    if u is None:
                        self.err("for の do が無い")
                    if u.kind == "WORD" and u.val == "do" and self.t[self.i - 1].kind in ("NL", "OP", "ARITH"):
                        break
                    self.i += 1
                return self.loop_body()
            if v == "case":
                self.i += 2  # case <語>
                self.skip_nl()
                self.expect_word("in")
                bodies = []
                while True:
                    self.skip_nl()
                    if self.is_word("esac"):
                        self.i += 1
                        break
                    while not (self.peek() and self.peek().kind == "OP" and self.peek().val == ")"):
                        if self.peek() is None:
                            self.err("case の ) が無い")
                        self.i += 1
                    self.i += 1
                    bodies.append(self.parse_list(words=("esac",), ops=(";;", ";&", ";;&")))
                    if self.peek() and self.peek().kind == "OP" and self.peek().val in (";;", ";&", ";;&"):
                        self.i += 1
                self.redirs()
                return Node(kind="if", bodies=bodies)
            if v == "function":
                self.i += 2
                if self.peek() and self.peek().kind == "OP" and self.peek().val == "(":
                    self.i += 2
                return self.func_body()
            nxt, nxt2 = self.peek(1), self.peek(2)
            if nxt and nxt.kind == "OP" and nxt.val == "(" and nxt2 and nxt2.kind == "OP" and nxt2.val == ")":
                self.i += 3
                return self.func_body()
        words = []
        while self.peek() and self.peek().kind in ("WORD", "REDIR", "TEST"):
            u = self.take()
            if u.kind == "REDIR":
                if u.target:
                    self.take()
            else:
                words.append(u.val)
        if not words and self.peek() is not None and self.peek().kind == "OP":
            self.err(f"{self.peek().val} の前にコマンドが無い")
        return Node(kind="simple", words=words)

    def loop_body(self):
        self.expect_word("do")
        body = self.parse_list(words=("done",))
        self.expect_word("done")
        self.redirs()
        return Node(kind="loop", body=body)

    def func_body(self):
        self.skip_nl()
        c = self.command()
        return Node(kind="func", body=c)


ASSIGN = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*(\[[^]]*\])?\+?=")


def name_of(c):
    """単純なコマンドの名前（パスは最後の段だけ）と引数。前に置いた代入（X=1 cmd）と道具（timeout 5 …）は外す。"""
    words = list(c.words)
    while True:
        while words and ASSIGN.match(words[0]):
            words.pop(0)
        if not words:
            return None, []
        name = re.sub(r"[\"'\\]", "", words[0]).rsplit("/", 1)[-1]
        if name in ("sh", "bash") and words[1:2] == ["-c"] and len(words) > 2:
            # bash -c '<コマンド> …' は、文字列の最後のコマンドの終了コードを返す。それで見る
            words = [x for x in re.split(r"[;&|\n]", re.sub(r"[\"']", "", words[2])) if x.strip()][-1:]
            words = words[0].split() if words else []
            continue
        if name not in WRAPPERS:
            return name, words[1:]
        words.pop(0)
        positional, valued = WRAPPERS[name]
        split = None
        while words and words[0].startswith("-") and split is None:
            flag = words.pop(0)
            # env -S '<コマンド> <引数>…' の値は、割ってコマンドとして読む（-S'…'・--split-string=… も）
            if name == "env" and flag in ("-S", "--split-string"):
                split, words = (words[0] if words else ""), words[1:]
            elif name == "env" and flag.startswith("--split-string="):
                split = flag[len("--split-string=") :]
            elif name == "env" and flag.startswith("-S"):
                split = flag[2:]
            elif flag in valued:
                words = words[1:]  # 値を取る旗（timeout -s KILL・env -u X・stdbuf -o L）
        if split is not None:
            words = re.sub(r"[\"']", "", split).split() + words
            continue
        words = words[positional:]


def has_stop(stmts):
    """並びのどこかに、止める文（die・fail・exit・return）があるか。関数の定義の中は数えない。"""
    for st in stmts:
        for p in st.items:
            for c in p.cmds:
                if c.kind == "simple" and name_of(c)[0] in ("die", "fail", "exit", "return"):
                    return True
                bodies = [c.body] if c.kind in ("group", "subshell", "loop") else c.bodies if c.kind == "if" else []
                if any(has_stop(b) for b in bodies):
                    return True
    return False


def is_check(p):
    """項が判定か。A && <判定> の A が偽だと、判定を飛ばして先へ進む。"""
    if p.neg:
        return True  # A && ! B は、B が真でも set -e が止めない
    c = p.cmds[-1]
    if c.kind == "test":
        return True
    if c.kind in ("group", "subshell"):
        # A && { grep …; grep …; } は判定のまとまり。A && { echo FAIL; exit 1; } は逆の判定で、拾わない
        if not c.body or has_stop(c.body):
            return False
        st = c.body[-1]
        return st.ops[-1:] != ["||"] and is_check(st.items[-1])
    if c.kind != "simple":
        return False
    name, args = name_of(c)
    if name in CHECKS or name in REPORTS:
        return True
    if name == "jq":
        return any(w == "--exit-status" or re.match(r"^-[a-zA-Z]*e[a-zA-Z]*$", w) for w in args)
    if name == "git":
        return bool(args) and args[0] == "diff" and ("--quiet" in args or "--exit-code" in args)
    return False


def talks(stmts):
    """並びが echo・printf だけか。"""
    return all(
        not st.ops and len(st.items[0].cmds) == 1 and st.items[0].cmds[0].kind == "simple"
        and name_of(st.items[0].cmds[0])[0] in TALKS
        for st in stmts
    )


def stops(p):
    """|| の受け手になっているか。|| echo "FAIL …" >&2 と || { echo …; printf …; } は言うだけで止めない。"""
    c = p.cmds[-1]
    if c.kind == "simple":
        return name_of(c)[0] not in TALKS
    if c.kind in ("group", "subshell"):
        return not talks(c.body)
    return True


def unguarded(st):
    """受け手の無い A && <判定> か。|| があっても、最後の演算子が && か、受け手が止めないなら受け手の無いのと同じ。"""
    if "&&" not in st.ops or st.bg:
        return False
    if st.ops[-1] == "&&":
        return is_check(st.items[-1])
    return not stops(st.items[-1]) and "&&" in st.ops[:-1] and is_check(st.items[-2])


def walk(stmts, tail, hits):
    """tail は、並びの最後の文の終了コードが外へ返る（関数かサブシェルの本体の最後）か。"""
    for k, st in enumerate(stmts):
        last = k == len(stmts) - 1
        if not (tail and last) and unguarded(st):
            hits.append(st.pos)
        # 1つだけのコマンドの文が最後なら、if・case・{ } の本体の最後も外へ返る
        alone = tail and last and not st.ops and len(st.items[0].cmds) == 1 and not st.items[0].neg
        for j, p in enumerate(st.items):
            # && と || の最後でない項と ! の中は、set -e が中身ごと効かない（( … ) || die のサブシェルの中も止まらない）。
            # そこでは A && B も単独の文も同じく止まらないので、見ない
            if j < len(st.items) - 1 or p.neg:
                continue
            for c in p.cmds:
                descend(c, alone, hits)


def descend(c, tail, hits):
    if c.kind == "subshell":
        walk(c.body, True, hits)
    elif c.kind == "group":
        walk(c.body, tail, hits)
    elif c.kind == "if":
        for b in c.bodies:
            walk(b, tail, hits)
    elif c.kind == "loop":
        walk(c.body, False, hits)
    elif c.kind == "func":
        b = c.body
        if b.kind in ("group", "subshell"):
            walk(b.body, True, hits)
        else:
            descend(b, True, hits)


def errexit(src):
    """set -e・set -eu・set -u -e・set -o errexit・set -o nounset -o errexit のどれかを書いたか。"""
    for m in re.finditer(r"^[ \t]*set[ \t]+([^\n;#]*)", src, re.M):
        args = m.group(1).split()
        for k, a in enumerate(args):
            if re.match(r"^-[a-zA-Z]*e[a-zA-Z]*$", a) or (a == "-o" and args[k + 1 : k + 2] == ["errexit"]):
                return True
    return False


bad = False
for path in sys.argv[1:]:
    with open(path, encoding="utf-8") as f:
        src = f.read()
    nl = [m.start() for m in re.finditer("\n", src)]
    line = lambda pos: bisect.bisect_left(nl, pos) + 1
    try:
        toks, _ = Lexer(src).tokens(0)
        parser = Parser(toks)
        stmts = parser.parse_list()
        if parser.peek() is not None:
            parser.err("読み残しがある")
    except ReadError as e:
        pos, msg = e.args[0]
        print(f"{path}:{line(pos) if pos >= 0 else '?'}: 読めない（{msg}）", file=sys.stderr)
        bad = True
        continue
    if not errexit(src):
        continue  # set -e を書いていないファイルは、判定を pass と fail で受ける
    hits = []
    walk(stmts, False, hits)
    lines = src.split("\n")
    for pos in sorted(set(hits)):
        n = line(pos)
        print(f"{path}:{n}: {lines[n - 1].strip()}")
sys.exit(2 if bad else 0)
PY
}

die() { echo "FAIL $*" >&2; exit 1; }

# check <ファイル> …: 拾った行を出して落ちる。読めないファイルがあっても、読めたファイルで拾った行は出す
check() {
  local out rc=0
  out=$(scan "$@") || rc=$?
  [ -z "$out" ] || printf '%s\n' "$out"
  [ "$rc" = 0 ] || die "読めないファイルがある（終了コード $rc）"
  [ -z "$out" ] || die "set -e で止まらない A && B の判定に、失敗の受け手が無い。判定なら行末に || die \"<何が起きなかったか>\" を付ける。A が偽なら B を見ずに先へ進む見張り（[ -n \"\$want\" ] && grep -q …）なら if A; then B; fi と書く"
}

if [ $# -gt 0 ]; then
  check "$@"
  echo "ok shell-and: $# ファイル"
  exit 0
fi

# 見本: 拾う形（行に「# 拾う」と書いた行）と拾わない形。here-doc の中なので、この検査自身の見る対象にはならない
cat >"$tmp/sample.sh" <<'SAMPLE'
#!/usr/bin/env bash
set -euo pipefail
die() { echo "FAIL $*" >&2; exit 1; }
refused() { ! "$@"; }
[[ $rc == 1 ]] && grep -q 'x' "$tmp/err" # 拾う
refused dry && grep -q "y" "$tmp/err" # 拾う
grep -q a f && ! grep -q b f # 拾う
[ "$(wc -l <"$f")" = 3 ] && [ -f "$d/a" ] && [ -f "$d/b" ] # 拾う
[ "$a" = '&&' ] && grep -q "x || y" f # 拾う
[ -f a ]&&grep -q b a # 拾う
[  -f a  ]  &&  grep  -q  b  a # 拾う
[ -f a ] && # 拾う（並びの始まりの行で出す）
  grep -q b a
[ -f a ] && jq -e .x a >/dev/null # 拾う
cmp -s a b && cmp -s c d # 拾う
[ -f a ] && [[ -s a ]] # 拾う
[ -f a ] && (( n > 1 )) # 拾う
X=1 [ -f a ] && LC_ALL=C grep -q b a 2>/dev/null # 拾う
for x in a b; do
  [ -f "$x" ] && [ -s "$x" ] # 拾う
done
if true; then
  [ -f a ] && grep -q b a # 拾う
else
  [ -f a ] && grep -q c a # 拾う
fi
case $x in
  a) [ -f x ] && grep -q y x ;; # 拾う
  *) : ;;
esac
{ [ -f a ] && grep -q b a; } # 拾う
case $x in
  a) [[ $rc == 1 || $rc == 2 ]] && grep -q y x ;; # 拾う（パターンの後ろの [[ … ]] の中の || は受け手でない）
  b*) [[ $x =~ ^a(b|c)$ ]] || exit 1 ;;
esac
[[ $rc == 1 ]] && ok "断る" # 拾う（成功を言うだけの項は判定の続き）
[[ $rc == 1 ]] && pass "断る" # 拾う
[[ $rc == 1 ]] && { grep -q x err; grep -q y err; } # 拾う
[[ $rc == 1 ]] && timeout 5 grep -q x err # 拾う
[[ $rc == 1 ]] && env LC_ALL=C grep -q x err # 拾う
[[ $rc == 1 ]] && env -S 'grep -q x' err # 拾う（-S の値はコマンドと引数）
[[ $rc == 1 ]] && env --split-string="grep -q x" err # 拾う
[[ $rc == 1 ]] && env -u HOME -S'LC_ALL=C grep -q x' err # 拾う
[[ $rc == 1 ]] && bash -c 'grep -q x err' # 拾う（-c の文字列の最後のコマンドで見る）
[[ $rc == 1 ]] && sh -c "cd d; grep -q x err" # 拾う
[[ $rc == 1 ]] && sh -c "grep -q x err && mv a b"
[[ $rc == 1 ]] && bash -c 'mv a b'
[[ $rc == 1 ]] && /usr/bin/grep -q x err # 拾う
[[ $rc == 1 ]] && git diff --quiet # 拾う
[[ $rc == 1 ]] || [[ $rc == 2 ]] && grep -q x err # 拾う（最後の演算子が &&）
[[ $rc == 1 ]] && grep -q x err || echo "FAIL rc=$rc" >&2 # 拾う（受け手が止まらない）
[[ $rc == 1 ]] && grep -q x err || { echo "FAIL rc=$rc" >&2; exit 1; }
[[ $rc == 1 ]] && grep -q x err || fail "x"
[[ $rc == 1 ]] && grep -q x err || true
[[ $rc == 1 ]] && { echo FAIL >&2; exit 1; }
[ -n "$want" ] && grep -q "$want" out # 拾う（判定と見分けられない。見張りは次の行の if で書く）
if [ -n "$want" ]; then grep -q "$want" out; fi
f() {
  [ -f a ] && grep -q b a # 拾う（関数の最後でない）
  :
}
[[ $rc == 1 ]] && grep -q 'x' "$tmp/err" || die "x"
[[ $rc == 1 ]] && grep -q 'x' "$tmp/err" ||
  die "x"
if [ -f a ] && grep -q b a; then :; fi
while [ -f a ] && grep -q b a; do break; done
until [ -f a ] && grep -q b a; do break; done
[ -n "$x" ] && echo "$x"
[ -e "$p" ] && [ "$p" != docs ] && ln -s "$PWD/$p" "$tmp/$p"
grep -q x f && die "x"
grep -q x f && { echo FAIL; exit 1; }
jq '.a' f >"$f.new" && mv "$f.new" f
[[ -f a && -s a ]]
g() {
  [ -f a ] && grep -q b a
}
h() { cd d && grep -q b a; }
( cd d && grep -q x f )
(
  [ -f a ] && grep -q b a
  :
) || die "x"
! { [ -f a ] && grep -q b a; :; }
y=$(cd d && grep -c x f)
z="$( [ -f a ] && grep -q b a; echo $? )"
cat <<EOF
[ -f a ] && grep -q b a
EOF
cat <<-'EOF' >"$tmp/x"
	[ -f a ] && grep -q b a
	EOF
# [ -f a ] && grep -q b a
echo '[ -f a ] && grep -q b a'
echo "[ -f a ] && grep -q b a"
[ -f a ] && grep -q b a &
wait
SAMPLE
# 見本の「# 拾う」の行が、拾う行のすべて
grep -n '# 拾う' "$tmp/sample.sh" | cut -d: -f1 | sed "s|^|$tmp/sample.sh:|" >"$tmp/want"
scan "$tmp/sample.sh" | cut -d: -f1-2 >"$tmp/got" || die "見本を読めない"
[ "$(wc -l <"$tmp/want")" -ge 15 ] || die "見本の拾う行が少なすぎる（$(wc -l <"$tmp/want") 行）"
diff "$tmp/want" "$tmp/got" >&2 || die "見本で拾う行が違う（< 拾うはずの行・> 拾った行）"
# set -e の無いファイルは見ない。set -o errexit は set -e と同じに見る
printf '#!/usr/bin/env bash\n[ -f a ] && grep -q b a\n' >"$tmp/no-e.sh"
printf '#!/usr/bin/env bash\nset -o errexit\n[ -f a ] && grep -q b a\n' >"$tmp/errexit.sh"
[ -z "$(scan "$tmp/no-e.sh")" ] || die "set -e の無いファイルを見た"
[ "$(scan "$tmp/errexit.sh" | cut -d: -f2)" = 3 ] || die "set -o errexit のファイルで拾わない"
for opts in '-u -e' '-o nounset -o errexit' '-eu'; do
  printf '#!/usr/bin/env bash\nset %s\n[ -f a ] && grep -q b a\n' "$opts" >"$tmp/opts.sh"
  [ "$(scan "$tmp/opts.sh" | cut -d: -f2)" = 3 ] || die "set $opts のファイルで拾わない"
done
# 読めない形は落ちる（黙って何も見ずに通らない）
printf '#!/usr/bin/env bash\nset -e\necho "x\n' >"$tmp/broken.sh"
if scan "$tmp/broken.sh" >/dev/null 2>"$tmp/err"; then die "閉じていない引用符のファイルを読めたことにした"; fi
grep -q '読めない' "$tmp/err" || die "読めないときの理由が出ない"
# 読めないファイルが混ざっても、読めたファイルで拾った行は出す
if (check "$tmp/errexit.sh" "$tmp/broken.sh") >"$tmp/out" 2>&1; then die "読めないファイルが混ざって通った"; fi
grep -q 'errexit.sh:3:' "$tmp/out" || die "読めないファイルが混ざると、拾った行が出ない"
echo 'ok shell-and: 見本の拾う形と拾わない形'

# tests/ の下で git が追跡するシェル
files=()
while IFS= read -r f; do
  case "$f" in
    *.sh) files+=("$f") ;;
    *) [ -f "$f" ] && head -1 "$f" | grep -qE '^#!.*[/ ](bash|sh)([[:space:]]|$)' && files+=("$f") || true ;;
  esac
done < <(git ls-files --cached --others --exclude-standard tests)
[ "${#files[@]}" -ge 50 ] || die "見るファイルが少なすぎる（${#files[@]} 本）"
check "${files[@]}"
echo "ok shell-and: tests/ の下のシェル ${#files[@]} 本に、受け手の無い A && B の判定が無い"
