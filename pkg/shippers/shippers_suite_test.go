package shippers_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestShippers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shippers Suite")
}
