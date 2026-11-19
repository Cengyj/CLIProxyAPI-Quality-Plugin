FROM golang:1.26-bookworm@sha256:dc40a13b61304647ed7e5d581d246b86413dfc29d63c6d018310feb93cd98fc0 AS build-base
ENV CGO_ENABLED=1 GOARM64=v8.0
WORKDIR /src

FROM build-base AS host-build
COPY host/ /src/host/
WORKDIR /src/host
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -buildvcs=false -ldflags="-s -w -X main.Version=8.0.76 -X main.Commit=source-archive" -o /out/CLIProxyAPI ./cmd/server

FROM debian:bookworm-slim@sha256:734a72b3d6142a71993b4bc580ca01d8ca687f77a2c422d96a961d3376b179bc AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*
ENV TZ=Asia/Shanghai
WORKDIR /CLIProxyAPI
COPY --from=host-build /out/CLIProxyAPI ./CLIProxyAPI
COPY host/config.example.yaml ./config.example.yaml
EXPOSE 8317
CMD ["./CLIProxyAPI", "--config", "/CLIProxyAPI/config.yaml", "--local-model"]

FROM build-base AS plugin-build
RUN apt-get update && apt-get install -y --no-install-recommends python3 && rm -rf /var/lib/apt/lists/*
COPY host/ /src/host/
COPY plugin/ /src/plugin/
COPY login-runtime-go/ /src/login-runtime-go/
COPY scripts/build_official_plugin.py /src/scripts/build_official_plugin.py
COPY build_plugin.py /src/build_plugin.py
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    python3 /src/build_plugin.py

FROM scratch AS plugin-artifacts
COPY --from=plugin-build /out/ /
