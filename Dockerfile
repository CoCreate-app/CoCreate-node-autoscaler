FROM golang:1.15 AS builder
WORKDIR /build/nodeautoscaler
COPY ./ .
RUN chmod +x ./nodeautoscaler/build.sh

FROM alpine:3.12.4
COPY --from=builder /build/nodeautoscaler/nodeautoscaler/nodeautoscaler /nodeautoscaler
RUN chmod +x /nodeautoscaler
ENTRYPOINT ["/nodeautoscaler"]
