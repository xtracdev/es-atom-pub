image:
	docker run --rm -v $(CURDIR)/../:/go/src/github.com/xtracdev/es-atom-pub -w /go/src/github.com/xtracdev/es-atom-pub xtracdev/goora bash -c "cd cmd; make -f Makefile.docker"
	docker build -t xtracdev/atompub:latest .
