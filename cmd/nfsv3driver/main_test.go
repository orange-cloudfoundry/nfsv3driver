package main_test

import (
	"io/ioutil"
	"net"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Main", func() {
	var (
		session *gexec.Session
		command *exec.Cmd
		err     error
	)

	BeforeEach(func() {
		command = exec.Command(driverPath)
	})

	JustBeforeEach(func() {
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		session.Kill().Wait()
	})

	Context("with a driver path", func() {
		BeforeEach(func() {
			dir, err := ioutil.TempDir("", "driversPath")
			Expect(err).ToNot(HaveOccurred())

			command.Args = append(command.Args, "-driversPath="+dir)
		})

		It("listens on tcp/7589 by default", func() {
			EventuallyWithOffset(1, func() error {
				_, err := net.Dial("tcp", "0.0.0.0:7589")
				return err
			}, 5).ShouldNot(HaveOccurred())
		})
	})

	Context("with a driver path & sourceAllowed flag", func() {
		BeforeEach(func() {
			dir, err := ioutil.TempDir("", "driversPath")
			Expect(err).ToNot(HaveOccurred())

			command.Args = append(command.Args, "-driversPath="+dir, "-sourceAllowed=\"uid,gid\"")
		})

		It("listens on tcp/7589 by default", func() {
			EventuallyWithOffset(1, func() error {
				_, err := net.Dial("tcp", "0.0.0.0:7589")
				return err
			}, 5).ShouldNot(HaveOccurred())
		})
	})

	Context("with a driver path & sourceDefault flag", func() {
		BeforeEach(func() {
			dir, err := ioutil.TempDir("", "driversPath")
			Expect(err).ToNot(HaveOccurred())

			command.Args = append(command.Args, "-driversPath="+dir, "-sourceDefault=\"uid:1000,gid:1000\"")
		})

		It("listens on tcp/7589 by default", func() {
			EventuallyWithOffset(1, func() error {
				_, err := net.Dial("tcp", "0.0.0.0:7589")
				return err
			}, 5).ShouldNot(HaveOccurred())
		})
	})

	Context("with a driver path & sourceAllowed flag", func() {
		BeforeEach(func() {
			dir, err := ioutil.TempDir("", "driversPath")
			Expect(err).ToNot(HaveOccurred())

			command.Args = append(command.Args, "-driversPath="+dir, "-sourceAllowed=\"sloppy_mount,nfs_uid,nfs_gid\"")
		})

		It("listens on tcp/7589 by default", func() {
			EventuallyWithOffset(1, func() error {
				_, err := net.Dial("tcp", "0.0.0.0:7589")
				return err
			}, 5).ShouldNot(HaveOccurred())
		})
	})

	Context("with a driver path & sourceAllowed flag", func() {
		BeforeEach(func() {
			dir, err := ioutil.TempDir("", "driversPath")
			Expect(err).ToNot(HaveOccurred())

			command.Args = append(command.Args, "-driversPath="+dir, "-sourceAllowed=\"sloppy_mount:true,nfs_uid:1000,nfs_gid:1000\"")
		})

		It("listens on tcp/7589 by default", func() {
			EventuallyWithOffset(1, func() error {
				_, err := net.Dial("tcp", "0.0.0.0:7589")
				return err
			}, 5).ShouldNot(HaveOccurred())
		})
	})
})
