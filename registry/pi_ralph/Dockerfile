# Build context: registry/pi_ralph
FROM node:22.22.1-bookworm-slim@sha256:4f77a690f2f8946ab16fe1e791a3ac0667ae1c3575c3e4d0d4589e9ed5bfaf3d

ARG PI_VERSION=1.0.3
ARG RALPH_VERSION=2.1.0

USER root
RUN apt-get update \
    && apt-get install --yes --no-install-recommends ca-certificates ripgrep \
    && rm -rf /var/lib/apt/lists/* \
    && npm install --global --ignore-scripts \
        "@earendil-works/pi-coding-agent@${PI_VERSION}" \
        "@lnilluv/pi-ralph-loop@${RALPH_VERSION}" \
    && npm cache clean --force

COPY launcher.mjs protocol.mjs telemetry.ts tools.mjs tools.ts patch-ralph.mjs /opt/watts/
COPY kit/skills /opt/watts/kit/skills/
RUN node /opt/watts/patch-ralph.mjs

ENV PI_CODING_AGENT_DIR=/runtime/pi

USER node
WORKDIR /workspace
ENTRYPOINT ["node", "/opt/watts/launcher.mjs"]
