FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /floki .

FROM alpine:3.20
COPY --from=build /floki /usr/local/bin/floki
# AOF file (./floki-db.aof) is written relative to the working dir -> mount a volume here
WORKDIR /data
VOLUME /data
EXPOSE 7379
ENTRYPOINT ["floki"]
CMD ["-host", "0.0.0.0", "-port", "7379"]
