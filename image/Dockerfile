# syntax=docker/dockerfile:1
# andon は uid・gid 1000（ubuntu の既定の利用者を外して空ける）。建屋の repo を /workspace に bind mount するので、
# ホストの持ち主（WSL・cloud-init の既定の利用者は 1000）と揃える。揃わない建屋は bin/bootstrap が起こす前に止める
FROM ubuntu:24.04@sha256:fe81560990fae86dc4743202b2cb4d3df98d0bcf949173cecef15ce1127ef852

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl git unzip openssh-server sudo locales libatomic1 bash-completion \
 && rm -rf /var/lib/apt/lists/* \
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
ENV ENABLE_TOOL_SEARCH=auto
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
 && mkdir -p "${HOME}/.local/bin"
COPY --chown=andon:andon --chmod=755 image/agent-wrapper /opt/andon-home/.local/bin/agent-wrapper
RUN ln -s agent-wrapper "${HOME}/.local/bin/portless"
ENV PLAYWRIGHT_BROWSERS_PATH=/opt/andon-home/.cache/ms-playwright
ARG INSTALL_BROWSERS=1
RUN if [ "${INSTALL_BROWSERS}" = 1 ]; then /opt/andon-tools/bin/playwright install --with-deps chromium; fi
ARG COOKIECUTTER_VERSION=2.7.1
ARG CRUFT_VERSION=2.16.0
RUN uv python install 3.14.10 --default \
 && uv tool install "cookiecutter==${COOKIECUTTER_VERSION}" \
 && uv tool install "cruft==${CRUFT_VERSION}" \
 && echo 'eval "$(mise activate bash)"' >> "${HOME}/.bashrc" \
 && echo 'eval "$(direnv hook bash)"' >> "${HOME}/.bashrc" \
 && echo '[ -r /etc/andon-bashrc ] && . /etc/andon-bashrc' >> "${HOME}/.bashrc"
ENV CLAUDE_CONFIG_DIR=/opt/andon-home/.claude
ARG CLAUDE_VERSION=2.1.318
ARG CLAUDE_SHA256_AMD64=708c77b204db30086ff6f1ed34577bda03e1be74ced80557cbf7b54249ebf0bd
ARG CLAUDE_SHA256_ARM64=53a0d7e219d7b0804ecba5dbc002745e89f786cc946d2de85a6470d5b2d1fb06
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

# 建屋の compose（package/compose.yaml）が volume を重ねる置き場。先に andon の持ち物で作っておく
# （無い場所に volume を重ねると root の持ち物になり、andon が書けない）
RUN mkdir -p "${HOME}/.claude" "${HOME}/.codex" "${HOME}/.agents" "${HOME}/.config/gh" "${HOME}/.config/herdr" \
      "${HOME}/.config/herdr-core" "${HOME}/.ssh" "${HOME}/.portless" "${HOME}/.local/state" \
 && chmod 700 "${HOME}/.ssh"
COPY package/ /opt/andon/
# skill の手順が JSON を確かめる schema（package/lib/andon/schema_check.py が読む。ANDON_SCHEMA の既定）
COPY schema/ /opt/andon/schema/
# 台帳の CLI（cmd/kanban）。tests/image.sh が同じ repo から焼いたものを build context `kanban` で渡す
# （docker build --build-context kanban=<dir>）。deck は1版だけの別名（docs/design.md §3.5 の K5）
COPY --from=kanban --chmod=755 kanban /usr/local/bin/kanban
# andon の CLI（cmd/andon）。kanban と同じく tests/image.sh が焼き、build context `andon` で渡す。
# compose の `andon boot` と、ホストが取り出す口（docs/operate.md の 2）がこれを使う
COPY --from=andon --chmod=755 andon /opt/andon/bin/andon
# 対話 bash の設定（~/.bashrc の末尾が読む）と Tab 補完。gh・just・mise は各 CLI が生成するもの、hc は --help の一覧から作る
# （hc に生成の口が無い）、andon・kanban・deck はサブコマンド名だけの小さな補完（image/completions/）
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
 && ln -s kanban /usr/local/bin/deck \
 && install -m 644 /tmp/andon-gen/* /tmp/andon-completions/* /usr/share/bash-completion/completions/ \
 && rm -rf /tmp/andon-gen /tmp/andon-completions
USER andon
WORKDIR /workspace
CMD ["bash"]
