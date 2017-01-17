FROM xtracdev/elcaro:0.2.0

COPY atompub /opt/
EXPOSE 4567
HEALTHCHECK CMD curl --noproxy localhost --fail http://localhost:4567/health || exit 1
ENTRYPOINT ["/opt/atompub"]
