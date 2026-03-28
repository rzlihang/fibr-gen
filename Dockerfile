# Frontend build stage
FROM node:22-alpine AS frontend
RUN corepack enable pnpm
WORKDIR /app/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ .
RUN pnpm build

# Backend build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server

# Runtime stage
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /server /server
COPY --from=frontend /app/web/dist /static
EXPOSE 8080
CMD ["/server"]
