FROM alpine:latest

RUN sed -i "s@dl-cdn.alpinelinux.org@mirrors.aliyun.com@g" /etc/apk/repositories

# 时区设置为上海时区
RUN apk add tzdata && cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone \
    && apk del tzdata

ENV CGO_ENABLED=0
WORKDIR /root
COPY ./dump-handler /bin/dump-handler

ENTRYPOINT  [ "/bin/prom_remote_biz" ]