package etcd_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestBackendEtcd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Backend Etcd Test Suite")
}
