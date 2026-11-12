"""pty-probe: 端末の program を pty で起動し、<秒> のあいだ画面に出た文字を ANSI を除いて標準出力に出して止める。

usage: python3 pty-probe.py <秒> <cwd> <program> [args...]
（stdin から読ませるときは python3 - <秒> <cwd> <program> …）

信頼の画面のような最初の画面を見るためのもの。打鍵はしない（端末の問い合わせ〈位置・色・属性〉にだけ答える）。
標準ライブラリだけ。tests/image.sh がセルのコンテナの中で使う。
"""

import fcntl
import os
import pty
import re
import select
import signal
import struct
import sys
import termios
import time

ANSWERS = {
    b"\x1b[6n": b"\x1b[1;1R",
    b"\x1b[c": b"\x1b[?62;22c",
    b"\x1b]10;?": b"\x1b]10;rgb:ffff/ffff/ffff\x07",
    b"\x1b]11;?": b"\x1b]11;rgb:0000/0000/0000\x07",
    b"\x1b[?u": b"\x1b[?0u",
}


def main(argv):
    secs, cwd, cmd = float(argv[1]), argv[2], argv[3:]
    pid, fd = pty.fork()
    if pid == 0:
        os.chdir(cwd)
        os.environ["TERM"] = "xterm-256color"
        os.execvp(cmd[0], cmd)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 50, 160, 0, 0))
    buf = b""
    end = time.time() + secs
    while time.time() < end:
        r, _, _ = select.select([fd], [], [], 0.3)
        if not r:
            continue
        try:
            d = os.read(fd, 65536)
        except OSError:
            break
        if not d:
            break
        buf += d
        for q, a in ANSWERS.items():
            if q in d:
                os.write(fd, a)
    try:
        os.kill(pid, signal.SIGKILL)
        os.waitpid(pid, 0)
    except OSError:
        pass
    text = re.sub(rb"\x1b\[[0-9;?<>=]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)|\x1b[()][A-Z0-9]|\x1b[=>78DEHM]", b" ", buf)
    text = re.sub(rb"[\x00-\x08\x0b-\x1f]", b"", text)
    sys.stdout.write(re.sub(r" {2,}", " ", text.decode("utf-8", "replace")) + "\n")


if __name__ == "__main__":
    main(sys.argv)
