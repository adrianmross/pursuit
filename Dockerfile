# Toolchain versions are devenv's (devenv tasks run dev:pins); CI fails on drift.
ARG NODE_VERSION=22.23.4
ARG GO_VERSION=1.26.10

# The binary go:embeds the dashboard and docs (internal/dashboard/dist), so
# they are built first with the same script CI and Vercel use.
FROM node:${NODE_VERSION}-bookworm-slim AS web
WORKDIR /src
# Copy and build in one layer: the web-app build renames directories aside,
# which overlayfs refuses (EXDEV) for files that came from an earlier layer.
RUN --mount=type=bind,target=/ctx cp -a /ctx/. . && sh scripts/vercel-build.sh

FROM golang:${GO_VERSION}-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/internal/dashboard/dist internal/dashboard/dist
RUN CGO_ENABLED=0 go build -o /pursuit ./cmd/pursuit

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /pursuit /pursuit
USER nonroot:nonroot
EXPOSE 8787
ENTRYPOINT ["/pursuit"]
CMD ["serve"]
