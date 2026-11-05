# syntax=docker/dockerfile:1
FROM ubuntu:24.04@sha256:fe81560990fae86dc4743202b2cb4d3df98d0bcf949173cecef15ce1127ef852

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl git unzip openssh-server sudo locales libatomic1 \
 && rm -rf /var/lib/apt/lists/* \
 && locale-gen en_US.UTF-8 \
 && useradd --create-home --home-dir /opt/andon-home --shell /bin/bash andon \
 && usermod -p '*' andon \
 && echo 'andon ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/andon \
 && chmod 440 /etc/sudoers.d/andon \
 && mkdir -p /run/sshd /opt/andon-tools /opt/andon-aqua /workspace \
 && chmod 755 /run/sshd \
 && chown andon:andon /opt/andon-tools /opt/andon-aqua /workspace

ENV HOME=/opt/andon-home
ENV LANG=en_US.UTF-8
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
 && echo 'eval "$(direnv hook bash)"' >> "${HOME}/.bashrc"
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
USER root
RUN ln -s /opt/andon/bin/andon /usr/local/bin/andon
USER andon
WORKDIR /workspace
CMD ["bash"]
