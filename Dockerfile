FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o /out/gs ./cmd/gs

FROM alpine:latest
COPY --from=build /out/gs /usr/local/bin/gs
WORKDIR /work
ENTRYPOINT ["gs"]