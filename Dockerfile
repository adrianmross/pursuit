# The binary go:embeds the dashboard and docs (internal/dashboard/dist), so
# they are built first with the same script CI and Vercel use.
FROM node:22-bookworm-slim AS web
WORKDIR /src
# Copy and build in one layer: the web-app build renames directories aside,
# which overlayfs refuses (EXDEV) for files that came from an earlier layer.
RUN --mount=type=bind,target=/ctx cp -a /ctx/. . && sh scripts/vercel-build.sh

FROM golang:1.26-bookworm AS build
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
