# Build the coesa command with cgo off so the result runs on the static image.
FROM golang:1.27 AS build

WORKDIR /src

# The module has no external requirements, so this layer just pins the module graph.
COPY go.mod ./
RUN go mod download

COPY . .
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /coesa ./cmd/coesa

FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=build /coesa /coesa

# Documentation only. The listening address is set with -addr, whose default is :8080.
EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/coesa"]
