FROM scratch

COPY build/app /usr/bin/app

ENTRYPOINT [ "/usr/bin/app" ]