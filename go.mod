module github.com/projectcalico/libcalico-go

go 1.13

require (
	github.com/beorn7/perks v0.0.0-20180321164747-3a771d992973 // indirect
	github.com/coreos/bbolt v1.3.3 // indirect
	github.com/coreos/etcd v3.3.8+incompatible
	github.com/coreos/go-semver v0.3.0
	github.com/coreos/go-systemd v0.0.0-20190719114852-fd7a80b32e1f // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/dgrijalva/jwt-go v3.0.0+incompatible // indirect
	github.com/go-playground/locales v0.12.1 // indirect
	github.com/go-playground/universal-translator v0.0.0-20170327191703-71201497bace // indirect
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/google/btree v0.0.0-20161005200959-925471ac9e21 // indirect
	github.com/gorilla/websocket v1.4.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.9.6 // indirect
	github.com/jonboulle/clockwork v0.1.0 // indirect
	github.com/kelseyhightower/envconfig v0.0.0-20180517194557-dd1402a4d99d
	github.com/leodido/go-urn v0.0.0-20181204092800-a67a23e1c1af // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/onsi/ginkgo v1.6.0
	github.com/onsi/gomega v1.4.2
	github.com/philhofer/fwd v1.0.0 // indirect
	github.com/pquerna/ffjson v0.0.0-20190813045741-dac163c6c0a9 // indirect
	github.com/projectcalico/go-json v0.0.0-20161128004156-6219dc7339ba // indirect
	github.com/projectcalico/go-yaml-wrapper v0.0.0-20191112210931-090425220c54
	github.com/prometheus/client_golang v0.0.0-20171005112915-5cec1d0429b0
	github.com/prometheus/client_model v0.0.0-20170216185247-6f3806018612 // indirect
	github.com/prometheus/common v0.0.0-20171104095907-e3fb1a1acd76 // indirect
	github.com/prometheus/procfs v0.0.0-20171017214025-a6e9df898b13 // indirect
	github.com/satori/go.uuid v1.2.0
	github.com/sirupsen/logrus v1.4.2
	github.com/soheilhy/cmux v0.1.4 // indirect
	github.com/tinylib/msgp v1.1.0 // indirect
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	github.com/ugorji/go v0.0.0-20171019201919-bdcc60b419d1 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	go.etcd.io/bbolt v1.3.3 // indirect
	golang.org/x/net v0.0.0-20190812203447-cdfb69ac37fc
	golang.org/x/time v0.0.0-20170420181420-c06e80d9300e // indirect
	gopkg.in/airbrake/gobrake.v2 v2.0.9 // indirect
	gopkg.in/gemnasium/logrus-airbrake-hook.v2 v2.1.2 // indirect
	gopkg.in/go-playground/assert.v1 v1.2.1 // indirect
	gopkg.in/go-playground/validator.v9 v9.27.0
	gopkg.in/tchap/go-patricia.v2 v2.2.6
	gopkg.in/yaml.v2 v2.2.5 // indirect
	k8s.io/api v0.0.0-20190718183219-b59d8169aab5
	k8s.io/apimachinery v0.0.0-20190612205821-1799e75a0719
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
	k8s.io/code-generator v0.0.0-20190814140513-6483f25b1faf
)

replace github.com/sirupsen/logrus => github.com/projectcalico/logrus v1.0.4-calico
