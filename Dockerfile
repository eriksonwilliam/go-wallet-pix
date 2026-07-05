# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12 AS runtime
COPY --from=build /out/api /api
EXPOSE 3000
USER nonroot:nonroot
ENTRYPOINT ["/api"]
