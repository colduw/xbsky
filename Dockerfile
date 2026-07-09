FROM golang:latest AS builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 go build main.go

FROM ubuntu:latest AS final

COPY --from=builder /app/ /app/

RUN apt update -y && apt upgrade -y && apt install -y ffmpeg

WORKDIR /app

EXPOSE 80/tcp 443/tcp

VOLUME ["/app/certs"]

ENTRYPOINT [ "./main" ]