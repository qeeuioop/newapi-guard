FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/guard .

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/guard /app/guard
COPY web /app/web
RUN mkdir -p /app/data
EXPOSE 9000
CMD ["/app/guard"]
