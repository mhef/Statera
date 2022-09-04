FROM golang:1.18.3-alpine3.15 as base
RUN mkdir /build
WORKDIR /build 
ENV CGO_ENABLED=0
COPY go.* .
RUN go mod download
RUN go mod verify

FROM base as build
RUN --mount=target=. \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o /out/main ./cmd/statera

# build final image
FROM alpine:3.13 as production
COPY --from=build /out/main /app/
WORKDIR /app
ENTRYPOINT ["/app/main"]