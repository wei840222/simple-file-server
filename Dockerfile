FROM node:22.16.0-bullseye-slim AS web-builder-base
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable
COPY ./web/ /src
WORKDIR /src

FROM web-builder-base AS web-prod-deps
RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --prod --frozen-lockfile

FROM web-builder-base AS web-builder
RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile
RUN pnpm run build

FROM golang:1.24.5-bookworm AS builder

WORKDIR /src

COPY go.* ./
RUN go mod download

COPY . ./

RUN go build -v -o simple-file-server

FROM debian:bookworm-slim

# update ca-certificates
RUN set -x && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*

ARG user=simple-file-server
ARG group=simple-file-server
ARG uid=10000
ARG gid=10001

# If you bind mount a volume from the host or a data container,
# ensure you use the same uid
RUN groupadd -g ${gid} ${group} \
    && useradd -l -u ${uid} -g ${gid} -m -s /bin/bash ${user}

USER ${user}

COPY --from=builder --chown=${uid}:${gid} /src/simple-file-server /usr/bin/simple-file-server
COPY --from=builder --chown=${uid}:${gid} /src/config/config.yaml /etc/simple-file-server/config.yaml
COPY --from=web-builder --chown=${uid}:${gid} /src/dist /usr/share/simple-file-server/html
RUN mkdir -p /usr/share/simple-file-server/data && chown ${uid}:${gid} /usr/share/simple-file-server/data

ENV LOG_COLOR=true
ENV LOG_LEVEL=info
ENV LOG_FORMAT=console
ENV GIN_MODE=release
ENV FILE_ROOT=/usr/share/simple-file-server/data/files
ENV FILE_WEB_ROOT=/usr/share/simple-file-server/html

EXPOSE 8080

ENTRYPOINT ["simple-file-server"]