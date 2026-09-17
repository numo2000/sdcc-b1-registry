# Immagine minimale per il programma di supporto demo/fake-clients (solo
# libreria standard, nessuna dipendenza esterna - vedi main.go).
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod .
COPY main.go .
RUN CGO_ENABLED=0 go build -o /fake-clients .

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /fake-clients ./fake-clients
CMD ["./fake-clients"]
