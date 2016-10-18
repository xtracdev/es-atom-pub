image:
	docker run --rm -v "$(CURDIR)":/go/src/github.com/xtraclabs/es-atom-feed-pub/cmd -w /go/src/github.com/xtraclabs/es-atom-feed-pub/cmd xtracdev/goora bash -c "make -f Makefile.docker"
	docker build -t xtracdev/atompub .
