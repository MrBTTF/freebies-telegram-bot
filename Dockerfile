FROM alpine:latest

COPY main /
COPY keys /keys

EXPOSE 8080

CMD ["./main"]