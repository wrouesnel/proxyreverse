FROM golang:1.18 AS build

RUN mkdir /build

WORKDIR /build

COPY ./ ./

RUN go run mage.go binary

RUN useradd -u 1001 app \
 && mkdir /config \
 && chown app:root /config

FROM scratch

COPY --from=build /build/pathfinderproxy /bin/pathfinderproxy
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /config /

ENV PATH=/bin:$PATH

ENTRYPOINT ["pathfinderproxy"]

EXPOSE \
    443/tcp
    80/tcp
    8443/tcp
    53/udp
    53/tcp

USER 1001
