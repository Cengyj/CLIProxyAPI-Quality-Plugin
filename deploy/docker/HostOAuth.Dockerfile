# Run from the repository root with a clean official CPA v8.0.76 source directory:
# docker buildx build --platform linux/amd64 -f deploy/docker/HostOAuth.Dockerfile \
#   --build-context cpa=/path/to/CLIProxyAPI-8.0.76 --output type=local,dest=./out .
FROM golang:1.26-bookworm@sha256:dc40a13b61304647ed7e5d581d246b86413dfc29d63c6d018310feb93cd98fc0 AS dependencies
ENV CGO_ENABLED=1 GOAMD64=v1 GOTOOLCHAIN=local
WORKDIR /src/workspaces/CLIProxyAPI-official
COPY --from=cpa /go.mod /go.sum ./
COPY --from=cpa /sdk/pluginapi/types.go sdk/pluginapi/types.go
COPY --from=cpa /sdk/pluginabi/types.go sdk/pluginabi/types.go
RUN sha256sum -c <<'EOF'
23a6a3b5242aa3f8880ac5887e1e8690cd0fb63b09c7e625d7e56340160a8298  go.mod
e3de7cb0c9a558b81ba50cb6b1b1bdd903867bb39bc1ee8454d62b386893d826  go.sum
3eed9395fddd0b5d43f82a030526e68847858a6dd60d1459ebb626c704a45f6d  sdk/pluginapi/types.go
bf513d7ce5bb8ad0aefd5bbb638a07929064018063461157e961af59dc575770  sdk/pluginabi/types.go
EOF

WORKDIR /src/plugin
COPY plugin/go.mod plugin/go.sum ./
COPY plugin/quality/*.go quality/
COPY plugin/quality/questiondata/*.txt quality/questiondata/
COPY plugin/cmd/model-quality/*.go cmd/model-quality/
COPY plugin/ui/ ui/
# Keep the verified modules in this layer so offline builds also work after cache eviction.
RUN go list -mod=mod -tags credential_ops_host_oauth -test -deps ./... >/dev/null && go mod verify

FROM dependencies AS build
COPY deploy/docker/build_host_oauth.py /src/build_host_oauth.py
RUN --network=none --mount=type=cache,target=/root/.cache/go-build \
    python3 /src/build_host_oauth.py

FROM scratch
COPY --from=build /out/ /
