build:
	go get github.com/Sirupsen/logrus
	export PKG_CONFIG_PATH=$GOPATH/src/github.com/xtraclabs/es-atom-pub/pkgconfig/
	go get github.com/rjeczalik/pkgconfig/cmd/pkg-config
	go get github.com/mattn/go-oci8
	go get github.com/xtracdev/goes
	go get github.com/gucumber/gucumber/cmd/gucumber
	go get github.com/stretchr/testify/assert
	go get github.com/armon/go-metrics
	go get github.com/xtracdev/orapub
	go get gopkg.in/DATA-DOG/go-sqlmock.v1
	go test
	gucumber
