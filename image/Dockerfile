# syntax=docker/dockerfile:1
# 焼くのは image/build.sh（tests/image.sh・CI・セルの bin/bootstrap rebuild --from が同じものを打つ）。
# kanban と andon は下の段で docker の中で焼く（ホストに go は要らない）。go の版は mise.toml・go.mod と揃える
FROM golang:1.27.3-bookworm@sha256:dc0963f32053e4bc451d24d87e3892d42d01021b61f093e98eb95c0352bb84b0 AS bin
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY embed.go ./
COPY cmd/ cmd/
COPY internal/ internal/
COPY init/ init/
COPY process/states/issue.yaml process/states/issue.yaml
# kanban --version に出す版（image/build.sh が commit の短い SHA を渡す）
ARG ANDON_VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${ANDON_VERSION}" -o /out/kanban ./cmd/kanban \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/andon ./cmd/andon

# andon は uid・gid 1000（ubuntu の既定の利用者を外して空ける）。セルの repo を /workspace に bind mount するので、
# ホストのオーナー（WSL・cloud-init の既定の利用者は 1000）と揃える。揃わないセルは bin/bootstrap が起動する前に止める
FROM ubuntu:24.04@sha256:fe81560990fae86dc4743202b2cb4d3df98d0bcf949173cecef15ce1127ef852

# apt の取得元: 既定は ubuntu の archive・security のまま（手元と release の build）。CI の tests の job だけ image/build.sh が
# APT_MIRROR（ANDON_APT_MIRROR から）を渡し、runner と同じ mirror へ向ける。向けた sources は playwright の段の apt が
# 使い終わってから元に戻す（image には元の sources が残る。tests/image.sh が見る）
ARG APT_MIRROR=
# 索引（/var/lib/apt/lists）は build の cache に置き、playwright の段の apt と分け合う（apt-get update は build の中で1回。層には残らない）。
# ミラーの索引とプールが一時的にずれると install が 404 で落ちる。落ちたら update（索引の読み直し）からやり直す。
# tzdata はセルの TZ（compose が .env から渡す）を引くため。noninteractive でないと地域を聞いて止まる。/etc/localtime は UTC のまま
RUN --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    if [ -n "$APT_MIRROR" ]; then \
      cp /etc/apt/sources.list.d/ubuntu.sources /etc/apt/ubuntu.sources.orig \
      && sed -i -E "s#^URIs: http://(archive|security)\.ubuntu\.com/ubuntu/?\$#URIs: ${APT_MIRROR}#" /etc/apt/sources.list.d/ubuntu.sources \
      && ! grep -Eq '^URIs: http://(archive|security)\.ubuntu\.com' /etc/apt/sources.list.d/ubuntu.sources; \
    fi \
 && grep '^URIs:' /etc/apt/sources.list.d/ubuntu.sources | sed 's/^URIs: /apt の取得元: /' >&2 \
 && for i in 1 2 3 4 5; do \
      apt-get -o Acquire::Retries=3 update \
      && DEBIAN_FRONTEND=noninteractive apt-get -o Acquire::Retries=3 install -y --no-install-recommends ca-certificates curl git unzip openssh-server sudo locales libatomic1 bash-completion tzdata \
      && break; \
      [ "$i" = 5 ] && exit 1; \
      echo "apt が失敗した。索引を読み直してやり直す（$i/5）" >&2; sleep $((i * 5)); \
    done \
 && date +%s >/var/lib/apt/lists/andon-updated-at \
 && locale-gen en_US.UTF-8 \
 && userdel --remove ubuntu \
 && groupadd --gid 1000 andon \
 && useradd --uid 1000 --gid 1000 --create-home --home-dir /opt/andon-home --shell /bin/bash andon \
 && usermod -p '*' andon \
 && echo 'andon ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/andon \
 && chmod 440 /etc/sudoers.d/andon \
 && mkdir -p /run/sshd /opt/andon-tools /opt/andon-aqua /workspace \
 && chmod 755 /run/sshd \
 && chown andon:andon /opt/andon-tools /opt/andon-aqua /workspace
# sshd のセッションに下の ENV を渡す（sshd は ENV を引き継がない）。値を変えたらこのファイルも直す（tests/sshd-env.sh が見る）
COPY image/sshd_config.d/andon.conf /etc/ssh/sshd_config.d/andon.conf

ENV HOME=/opt/andon-home
ENV LANG=en_US.UTF-8
ENV SHELL=/bin/bash
ENV AQUA_ROOT_DIR=/opt/andon-home/.local/share/aquaproj-aqua
ENV PATH=/opt/andon-home/.local/bin:/opt/andon-home/.local/share/aquaproj-aqua/bin:${PATH}
ENV AQUA_GLOBAL_CONFIG=/opt/andon-aqua/aqua.yaml
ENV AQUA_POLICY_CONFIG=/opt/andon-aqua/aqua-policy.yaml
ENV GHQ_ROOT=/workspace/.ghq
ENV CLAUDE_CODE_NO_FLICKER=1
USER andon

ARG AQUA_INSTALLER_VERSION=v4.0.5
ARG AQUA_INSTALLER_SHA256=451028d56959cc738564885b1dbebc2691ea038ffde04e2472e4d486a3591146
ARG AQUA_VERSION=v2.62.3
RUN curl -fsSL -o /tmp/aqua-installer "https://raw.githubusercontent.com/aquaproj/aqua-installer/${AQUA_INSTALLER_VERSION}/aqua-installer" \
 && echo "${AQUA_INSTALLER_SHA256}  /tmp/aqua-installer" | sha256sum -c - \
 && bash /tmp/aqua-installer -v "${AQUA_VERSION}" \
 && rm /tmp/aqua-installer
COPY --chown=andon:andon image/aqua/ /opt/andon-aqua/
RUN aqua install --all

ARG PLAYWRIGHT_VERSION=1.62.1
ARG PORTLESS_VERSION=0.15.5
RUN npm config set prefix /opt/andon-tools \
 && npm install -g "playwright@${PLAYWRIGHT_VERSION}" "portless@${PORTLESS_VERSION}" \
 && npm cache clean --force \
 && rm -rf "${HOME}/.npm" \
 && mkdir -p "${HOME}/.local/bin" "${HOME}/.playwright"
COPY --chown=andon:andon --chmod=755 image/agent-wrapper /opt/andon-home/.local/bin/agent-wrapper
RUN ln -s agent-wrapper "${HOME}/.local/bin/portless" \
 && ln -s agent-wrapper "${HOME}/.local/bin/playwright"
# 担当が画面を見て確かめる道具（ADR-0018）: playwright-cli は `playwright cli` の wrapper。
# 設定は ~/.playwright/cli.config.json（cwd に依らず効く全体の設定。~/.playwright は volume に無いので image に焼く）:
# ブラウザを焼いた Chromium に（既定は Google Chrome を探して落ちる）、出力を worktree の外の /tmp/playwright-cli に（既定は cwd の .playwright-cli/）
COPY --chown=andon:andon --chmod=755 image/playwright-cli /opt/andon-home/.local/bin/playwright-cli
COPY --chown=andon:andon image/playwright/cli.config.json /opt/andon-home/.playwright/cli.config.json
ENV PLAYWRIGHT_BROWSERS_PATH=/opt/andon-home/.cache/ms-playwright
ARG INSTALL_BROWSERS=1
RUN if [ "${INSTALL_BROWSERS}" = 1 ]; then /opt/andon-tools/bin/playwright install chromium; fi
# Chromium の依存（playwright install-deps。--with-deps と同じ顔ぶれ）は、apt の段が取った索引で入れる。playwright は
# 必ず apt-get update を打つので、この RUN だけ update を飛ばす apt-get を PATH の先に置く（root なので playwright は sudo を通さず sh -c で打つ）。
# 飛ばすのは、索引の cache の andon-updated-at（apt の段が update した時刻）が1時間以内のときだけ。手元で apt の段が層の cache から来ると
# 索引は前の build のもので、古ければ update する（古い索引のまま古い依存を入れない）。索引が無い・取得元と合わず install が落ちたら update からやり直す。
# 最後に apt の取得元を元に戻す（APT_MIRROR を渡したときだけ控えがある）
USER root
RUN --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    if [ "${INSTALL_BROWSERS}" = 1 ]; then \
      mkdir /tmp/andon-apt \
      && printf '%s\n' '#!/bin/sh' \
           'if [ "$1" = update ]; then echo "apt-get update を飛ばす（索引は apt の段で取った）" >&2; exit 0; fi' \
           'exec /usr/bin/apt-get "$@"' >/tmp/andon-apt/apt-get \
      && chmod 755 /tmp/andon-apt/apt-get \
      && at=$(cat /var/lib/apt/lists/andon-updated-at 2>/dev/null || echo 0) \
      && if [ $(( $(date +%s) - at )) -lt 3600 ]; then skip=/tmp/andon-apt:; \
         else skip=; echo "索引が1時間より古い（apt の段が層の cache から来た）。apt-get update から取る" >&2; fi \
      && { PATH=$skip$PATH /opt/andon-tools/bin/playwright install-deps chromium \
           || { echo "apt の段の索引で入らなかった。apt-get update からやり直す" >&2; /opt/andon-tools/bin/playwright install-deps chromium; }; } \
      && rm -rf /tmp/andon-apt; \
    fi \
 && if [ -f /etc/apt/ubuntu.sources.orig ]; then mv /etc/apt/ubuntu.sources.orig /etc/apt/sources.list.d/ubuntu.sources; fi
USER andon
ARG COOKIECUTTER_VERSION=2.7.1
ARG CRUFT_VERSION=2.16.0
RUN uv python install 3.14.10 --default \
 && uv tool install "cookiecutter==${COOKIECUTTER_VERSION}" \
 && uv tool install "cruft==${CRUFT_VERSION}" \
 && echo 'eval "$(mise activate bash)"' >> "${HOME}/.bashrc" \
 && echo 'eval "$(direnv hook bash)"' >> "${HOME}/.bashrc" \
 && echo '[ -r /etc/andon-bashrc ] && . /etc/andon-bashrc' >> "${HOME}/.bashrc"
ENV CLAUDE_CONFIG_DIR=/opt/andon-home/.claude
ARG CLAUDE_VERSION=2.1.322
ARG CLAUDE_SHA256_AMD64=bfb3703ad5380242328459bc62ada4bde8ae7ba57c6e1eeadb9c00e08874c478
ARG CLAUDE_SHA256_ARM64=f220ca50ea4f3222043ff83cc224a1262e0196e9897acc0871efa94235b0c1fd
RUN case "$(dpkg --print-architecture)" in \
      amd64) platform=linux-x64; checksum="${CLAUDE_SHA256_AMD64}" ;; \
      arm64) platform=linux-arm64; checksum="${CLAUDE_SHA256_ARM64}" ;; \
      *) exit 1 ;; \
    esac \
 && curl -fsSL "https://downloads.claude.ai/claude-code-releases/${CLAUDE_VERSION}/${platform}/claude" -o /tmp/claude \
 && echo "${checksum}  /tmp/claude" | sha256sum -c - \
 && mkdir -p "${HOME}/.local/share/claude/versions" \
 && install -m 755 /tmp/claude "${HOME}/.local/share/claude/versions/${CLAUDE_VERSION}" \
 && ln -s "${HOME}/.local/share/claude/versions/${CLAUDE_VERSION}" "${HOME}/.local/bin/claude" \
 && rm /tmp/claude

# セルの compose（package/compose.yaml）が volume を重ねる置き場。先に andon の持ち物で作っておく
# （無い場所に volume を重ねると root の持ち物になり、andon が書けない）
RUN mkdir -p "${HOME}/.claude" "${HOME}/.codex" "${HOME}/.agents" "${HOME}/.config/gh" "${HOME}/.config/herdr" \
      "${HOME}/.config/herdr-core" "${HOME}/.config/andon/signals" "${HOME}/.ssh" "${HOME}/.portless" "${HOME}/.local/state" \
 && chmod 700 "${HOME}/.ssh" "${HOME}/.config/andon/signals"
COPY package/ /opt/andon/
# skill の手順が JSON を確かめる schema（package/lib/andon/schema_check.py が読む。ANDON_SCHEMA の既定）
COPY schema/ /opt/andon/schema/
# 台帳の CLI（cmd/kanban）。上の段 bin が同じ checkout から焼く。
COPY --from=bin --chmod=755 /out/kanban /usr/local/bin/kanban
# andon の CLI（cmd/andon）。kanban と同じく段 bin が焼く。
# compose の `andon boot` と、ホストが取り出す口（docs/operate.md の 2）がこれを使う
COPY --from=bin --chmod=755 /out/andon /opt/andon/bin/andon
# 対話 bash の設定（~/.bashrc の末尾が読む）と Tab 補完。gh・just・mise は各 CLI が生成するもの、hc は --help の一覧から作る
# （hc に生成の口が無い）、andon・kanban はサブコマンド名だけの小さな補完（image/completions/）
COPY --chmod=644 image/bashrc /etc/andon-bashrc
COPY --chmod=644 image/completions/ /tmp/andon-completions/
# 生成は andon で（aqua の道具は andon の持ち物。root が走らせると cache が root の持ち物になる）
RUN mkdir /tmp/andon-gen \
 && gh completion -s bash > /tmp/andon-gen/gh \
 && just --completions bash > /tmp/andon-gen/just \
 && mise completion bash > /tmp/andon-gen/mise \
 && words=$(hc --help | awk '/^  [a-z]/ && $1 != "hc" {print $1}' | sort -u | tr '\n' ' ') \
 && test -n "$words" \
 && printf '_hc() {\n  [ "$COMP_CWORD" -eq 1 ] || return 0\n  COMPREPLY=($(compgen -W "%s" -- "${COMP_WORDS[COMP_CWORD]}"))\n}\ncomplete -F _hc hc\n' "$words" > /tmp/andon-gen/hc
USER root
RUN ln -s /opt/andon/bin/andon /usr/local/bin/andon \
 && install -m 644 /tmp/andon-gen/* /tmp/andon-completions/* /usr/share/bash-completion/completions/ \
 && rm -rf /tmp/andon-gen /tmp/andon-completions
USER andon
WORKDIR /workspace
CMD ["bash"]
# 焼いた元の commit（image/build.sh が渡す。汚れた checkout なら末尾に -dirty）。bin/bootstrap status が出す
ARG ANDON_REVISION=unknown
LABEL org.opencontainers.image.revision=${ANDON_REVISION}
# 同じ commit を /opt/andon/version.json に焼く（andon version が読む。ADR-0028 P0-1）。中身の digest も数えるので、
# /opt/andon に書く段はこれより前に置く。commit の形でない revision は unknown として焼く（andon version は食い違いと出す）
USER root
RUN ANDON_HOME=/opt/andon /opt/andon/bin/andon version stamp "${ANDON_REVISION}" \
 || { echo "revision ${ANDON_REVISION} は commit の形でない。unknown として焼く" >&2; ANDON_HOME=/opt/andon /opt/andon/bin/andon version stamp unknown; }
USER andon
