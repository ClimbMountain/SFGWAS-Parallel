module github.com/hhcho/sfgwas-private

go 1.18

require (
	github.com/BurntSushi/toml v0.4.1
	github.com/aead/chacha20 v0.0.0-20180709150244-8b13a72661da
	github.com/hhcho/frand v1.3.1-0.20210217213629-f1c60c334950
	github.com/hhcho/mpc-core v0.0.0-20210527211839-87c954bf6638
	github.com/ldsec/lattigo/v2 v2.3.0
	github.com/ldsec/unlynx v1.4.3
	go.dedis.ch/onet/v3 v3.2.10
	gonum.org/v1/gonum v0.9.3
	gonum.org/v1/plot v0.9.0
)

require (
	github.com/ajstarks/svgo v0.0.0-20180226025133-644b8db467af // indirect
	github.com/daviddengcn/go-colortext v1.0.0 // indirect
	github.com/fanliao/go-concurrentMap v0.0.0-20141114143905-7d2d7a5ea67b // indirect
	github.com/fogleman/gg v1.3.0 // indirect
	github.com/go-fonts/liberation v0.1.1 // indirect
	github.com/go-latex/latex v0.0.0-20210118124228-b3d85cf34e07 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gopherjs/gopherjs v1.17.2 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/jtolds/gls v4.20.0+incompatible // indirect
	github.com/montanaflynn/stats v0.6.6 // indirect
	github.com/phpdave11/gofpdf v1.4.2 // indirect
	github.com/smartystreets/assertions v1.13.0 // indirect
	go.dedis.ch/fixbuf v1.0.3 // indirect
	go.dedis.ch/kyber/v3 v3.0.13 // indirect
	go.dedis.ch/protobuf v1.0.11 // indirect
	go.etcd.io/bbolt v1.3.6 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519 // indirect
	golang.org/x/image v0.0.0-20210216034530-4410531fe030 // indirect
	golang.org/x/sys v0.0.0-20211117180635-dee7805ff2e1 // indirect
	golang.org/x/text v0.3.5 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	rsc.io/goversion v1.2.0 // indirect
)

replace github.com/ldsec/lattigo/v2 => ./lattigo

replace github.com/hhcho/mpc-core => ./mpc-core
