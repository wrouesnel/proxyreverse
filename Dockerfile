FROM golang:1.23 AS build

RUN mkdir /build

WORKDIR /build

COPY ./ ./

RUN go run mage.go binary

RUN useradd -u 1001 app \
 && mkdir /config \
 && chown app:root /config

FROM scratch

COPY --from=build /build/proxyreverse /bin/proxyreverse
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /config /

ENV PATH=/bin:$PATH

ENTRYPOINT ["proxyreverse"]

USER 1001

CMD ["reverse-proxy"]