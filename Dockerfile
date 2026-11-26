# syntax=docker/dockerfile:1

# Toolchain suportado (as duas versoes mais recentes recebem correcao de
# seguranca). Com 1.24 o binario carregava 23 CVEs HIGH da stdlib.
FROM golang:1.27-alpine AS build
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
