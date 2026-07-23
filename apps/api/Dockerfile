FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/launchpad-api ./apps/api/cmd/api

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /bin/launchpad-api /app/launchpad-api
EXPOSE 8080
ENTRYPOINT ["/app/launchpad-api"]
