FROM alpine:latest

ENV HOME=/home/student

RUN mkdir -p $HOME && chmod 777 $HOME
WORKDIR $HOME

RUN apk add --no-cache \
    build-base \
    wget \
    python3

WORKDIR /tmp

# "Pin" sqlite to 3.53 until alpine includes it in their packages.
# A _lot_ of changes were made in 3.53, including a bunch of changes
# to formatting, which when you have tests that rely on specifically
# formatted output, is a problem. Students will need to make sure to
# have 3.53 installed locally as well if they want to be able to pass
# `make` as well as `grind grade`.
RUN wget https://www.sqlite.org/2026/sqlite-autoconf-3530000.tar.gz \
 && tar xzf sqlite-autoconf-3530000.tar.gz \
 && cd sqlite-autoconf-3530000 \
 && ./configure --prefix=/usr/local \
 && make -j$(nproc) \
 && make install \
 && rm -rf /tmp/*

ENV PATH="/usr/local/bin:$PATH"

WORKDIR $HOME
