# syntax=docker/dockerfile:1
{{- $BASE_IMAGE_NAME := getenv "BASE_IMAGE_NAME" "ubuntu" }}
{{- $BASE_IMAGE_TAG := getenv "BASE_IMAGE_TAG" "jammy" }}
ARG IMAGE_NAME="rewardenv/php-fpm"
ARG BASE_IMAGE_NAME="{{ $BASE_IMAGE_NAME }}"
ARG BASE_IMAGE_TAG="{{ $BASE_IMAGE_TAG }}"
ARG PHP_VERSION
ARG PHP_VARIANT="shopware"

FROM golang:alpine AS builder-supervisor

WORKDIR /src/

RUN <<-EOF
    set -eux
    apk add --no-cache \
      git \
      gcc \
      rust
    git clone https://github.com/ochinchina/supervisord.git .
    if [ "$(apk --print-arch)" = "aarch64" ]; \
      then BUILD_ARCH="arm64"; \
      else BUILD_ARCH="amd64"; \
    fi
    CGO_ENABLED=0 GOOS=linux GOARCH=$BUILD_ARCH go build -a -ldflags "-linkmode internal -extldflags -static" -o /usr/local/bin/supervisord github.com/ochinchina/supervisord
EOF

FROM ${IMAGE_NAME}:${PHP_VERSION}-${PHP_VARIANT}-${BASE_IMAGE_NAME}-${BASE_IMAGE_TAG}
USER root

ENV CRON_ENABLED            false
ENV SOCAT_ENABLED           false
ENV GOTTY_ENABLED           true
ENV GOTTY_USERNAME          shopware
ENV GOTTY_PASSWORD          shopware
ENV CHOWN_DIR_LIST          wp-content/uploads
ENV UID                     1000
ENV GID                     1000

ENV NGINX_UPSTREAM_HOST           127.0.0.1
ENV NGINX_UPSTREAM_PORT           9000
ENV NGINX_UPSTREAM_DEBUG_HOST     php-debug
ENV NGINX_UPSTREAM_DEBUG_PORT     9000
ENV NGINX_UPSTREAM_BLACKFIRE_HOST php-blackfire
ENV NGINX_UPSTREAM_BLACKFIRE_PORT 9000
ENV NGINX_ROOT                    /var/www/html
ENV NGINX_PUBLIC                  '/public'
ENV NGINX_TEMPLATE                shopware.conf
ENV XDEBUG_CONNECT_BACK_HOST      '""'
ENV SUDO_ENABLED                  "false"
ENV WWWDATA_PASSWORD              ""

COPY rootfs/. /
COPY --from=builder-supervisor /usr/local/bin/supervisord /usr/bin/

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

RUN <<-EOF
    set -eux
    apt-get update
    apt-get install -y --no-install-recommends \
      gnupg2
    echo "deb https://nginx.org/packages/{{ $BASE_IMAGE_NAME }}/ $(lsb_release -sc) nginx" >/etc/apt/sources.list.d/nginx.list
    wget -q -O - https://nginx.org/keys/nginx_signing.key | apt-key add -
    apt-get update
    apt-get install -y --no-install-recommends \
      nginx \
      netcat-openbsd
    BUILD_ARCH="$(dpkg --print-architecture)"
    if [ "${BUILD_ARCH}" = "arm64" ]; \
      then GOTTY_ARCH="arm64"; \
      else GOTTY_ARCH="amd64"; \
    fi
    wget -q -O /tmp/gotty.tar.gz \
      "https://github.com/sorenisanerd/gotty/releases/download/v1.5.0/gotty_v1.5.0_linux_${GOTTY_ARCH}.tar.gz"
    tar -zxvf /tmp/gotty.tar.gz -C /usr/bin
    rm -f /tmp/gotty.tar.gz
    rm -rf /var/lib/apt/lists/* /var/log/apt
    usermod -aG $GID nginx
    rm -f /etc/supervisor/supervisord.conf.dpkg-dist
    mkdir -p /etc/supervisor/conf.d
    chmod +x /usr/local/bin/check-dependencies.sh /usr/local/bin/install.sh /usr/local/bin/stop-supervisor.sh
    chown -R www-data: /etc/supervisor /etc/nginx /etc/php /var/log/nginx /var/cache/nginx /var/lib/php
    chmod +x /docker-entrypoint.sh
    ln -sf /dev/stdout /var/log/nginx/access.log && ln -sf /dev/stderr /var/log/nginx/error.log
EOF

USER www-data

EXPOSE 4200
EXPOSE 8080

CMD ["supervisord", "-c", "/etc/supervisor/supervisord.conf"]
