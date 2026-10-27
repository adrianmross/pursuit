FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /pursuit ./cmd/pursuit

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /pursuit /pursuit
USER nonroot:nonroot
EXPOSE 8787
ENTRYPOINT ["/pursuit"]
CMD ["serve"]
