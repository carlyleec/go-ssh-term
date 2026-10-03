FROM oven/bun:1.4.2 AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/bun.lock ./
RUN bun install --frozen-lockfile
COPY frontend/ ./
RUN bun run build

FROM golang:1.27.0 AS backend
WORKDIR /src
ENV GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY db/migrations/ ./db/migrations/
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server

FROM alpine:3.23.0
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=backend /out/server ./server
COPY --from=frontend /src/frontend/dist ./frontend/dist
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app/server"]
