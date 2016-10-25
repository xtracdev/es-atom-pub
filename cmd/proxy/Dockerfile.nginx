FROM nginx:1.11.5

COPY apt.conf /etc/apt/apt.conf

RUN apt-get update \
    && apt-get install -y netcat

COPY wait_for_feed_server.sh /opt/

ENTRYPOINT ["/opt/wait_for_feed_server.sh","atomfeedpub","8000"]