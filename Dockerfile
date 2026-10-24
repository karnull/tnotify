FROM debian:trixie-slim@sha256:62bd423aeee912639ca8ee681c2b762cb3e25137e0a92b20be455898a9f034ad AS base

# Configuration
ENV DEBIAN_FRONTEND=noninteractive
USER root

# Install dependencies
RUN apt update && apt upgrade -y \
    && apt install --no-install-recommends -y \
        ca-certificates \
        bash \
        make \
    && apt clean \
    && apt autoremove \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN groupadd --gid 1000 user \
    && useradd \
        --uid 1000 \
        --gid user \
        --create-home \
        --home-dir /home/user \
        --shell /bin/bash \
        --skel /etc/skel \
        user \
    && passwd -d user \
    && chown -R user:user /home/user

# Switch to non-root user
USER user
WORKDIR /home/user

COPY --from=golang:1.27.2-trixie@sha256:47667934874913a449b96faa7ef4777fa285d9ec29126ee80ed61c45b7cccf0e /usr/local/go /usr/local/go
ENV PATH=$PATH:/usr/local/go/bin
RUN go version

# Reset frontend to interactive
ENV DEBIAN_FRONTEND=dialog

