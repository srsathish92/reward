# syntax=docker/dockerfile:1
{{- $BASE_IMAGE_NAME := getenv "BASE_IMAGE_NAME" "ubuntu" }}
{{- $BASE_IMAGE_TAG := getenv "BASE_IMAGE_TAG" "jammy" }}
ARG IMAGE_NAME="rewardenv/php-fpm"
ARG BASE_IMAGE_NAME="{{ $BASE_IMAGE_NAME }}"
ARG BASE_IMAGE_TAG="{{ $BASE_IMAGE_TAG }}"
ARG PHP_VERSION
FROM ${IMAGE_NAME}:${PHP_VERSION}-${BASE_IMAGE_NAME}-${BASE_IMAGE_TAG}

USER root

# Resolve permission issues stemming from directories auto-created by docker due to mounts in sub-directories
ENV CHOWN_DIR_LIST "pub/media"
ENV SUDO_ENABLED   "true"

SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN <<-EOF
    set -eux
    npm install -g \
      grunt-cli \
      gulp \
      yarn
    PHP_VERSION_STRIPPED="$(echo "$PHP_VERSION" | awk -F '.' '{print $1$2}')"
    if [ "${PHP_VERSION_STRIPPED}" -le 71 ]; then \
      MAGERUN_PHAR_URL=https://raw.githubusercontent.com/rewardenv/magerun-mirror/main/n98-magerun2-3.2.0.phar; \
      MAGERUN_BASH_REF=3.2.0; \
    elif [ "${PHP_VERSION_STRIPPED}" -eq 72 ]; then \
      MAGERUN_PHAR_URL=https://raw.githubusercontent.com/rewardenv/magerun-mirror/main/n98-magerun2-4.7.0.phar; \
      MAGERUN_BASH_REF=4.7.0; \
    elif [ "${PHP_VERSION_STRIPPED}" -eq 73 ]; then \
      MAGERUN_PHAR_URL=https://raw.githubusercontent.com/rewardenv/magerun-mirror/main/n98-magerun2-6.1.1.phar; \
      MAGERUN_BASH_REF=6.1.1; \
    else \
      MAGERUN_PHAR_URL=https://raw.githubusercontent.com/rewardenv/magerun-mirror/main/n98-magerun2-latest.phar; \
      MAGERUN_BASH_REF=master; \
    fi
    curl -fsSLo /usr/local/bin/n98-magerun ${MAGERUN_PHAR_URL}
    chmod +x /usr/local/bin/n98-magerun
    curl -fsSLo /etc/bash_completion.d/n98-magerun2.phar.bash \
      https://raw.githubusercontent.com/netz98/n98-magerun2/${MAGERUN_BASH_REF}/res/autocompletion/bash/n98-magerun2.phar.bash
    perl -pi -e 's/^(complete -o default .*)$/$1 n98-magerun/' /etc/bash_completion.d/n98-magerun2.phar.bash
    # Create mr alias for n98-magerun
    ln -fs /usr/local/bin/n98-magerun /usr/local/bin/mr
EOF

USER www-data
