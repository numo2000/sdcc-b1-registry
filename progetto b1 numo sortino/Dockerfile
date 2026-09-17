# ---- Stage 1: build ----
# Immagine con il toolchain Go completo, usata solo per compilare il
# binario. Non finisce nell'immagine finale (multi-stage build).
FROM golang:1.25-alpine AS builder

WORKDIR /app

# go.mod/go.sum cambiano raramente rispetto al codice sorgente: copiarli e
# scaricare le dipendenze PRIMA di copiare il resto permette a Docker di
# riusare questo layer dalla cache ad ogni rebuild, finche' le dipendenze
# non cambiano davvero (vedi ARCHITETTURA.md non serve, ma stesso principio
# discusso per i layer Docker).
COPY go.mod go.sum ./
RUN go mod download

# Solo ora il resto del codice sorgente (cmd/, internal/) - un cambiamento
# qui non invalida il layer "go mod download" sopra.
COPY . .

# CGO_ENABLED=0: binario staticamente linkato, nessuna dipendenza da
# librerie C di sistema - puo' girare anche su un'immagine finale
# minimale come alpine senza problemi di compatibilita'.
RUN CGO_ENABLED=0 go build -o /registry-node ./cmd/registry-node

# ---- Stage 2: immagine finale, minimale ----
# alpine invece di scratch: pochi MB in piu', ma mantiene una shell
# disponibile per debug (docker exec -it ... /bin/sh) durante lo sviluppo.
FROM alpine:3.20

WORKDIR /app

COPY --from=builder /registry-node ./registry-node

# Documenta le porte di default (HTTP client-facing e gRPC gossip) - i
# valori reali vengono dal config.yaml montato a runtime (vedi CMD sotto),
# mai incorporati nell'immagine.
EXPOSE 7000 8000

# configs/config.yaml non viene copiato nell'immagine (vedi .dockerignore):
# ogni nodo lo riceve via bind mount da docker-compose.yml, cosi' la
# stessa immagine puo' servire N nodi con configurazioni diverse.
CMD ["./registry-node", "--config", "configs/config.yaml"]
