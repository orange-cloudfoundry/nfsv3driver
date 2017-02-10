package nfsv3driver_test

import (
	"context"
	"fmt"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"code.cloudfoundry.org/nfsdriver"
	"code.cloudfoundry.org/nfsv3driver"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"code.cloudfoundry.org/voldriver/voldriverfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strings"
)

var _ = Describe("NfsV3Mounter", func() {

	var (
		logger      lager.Logger
		testContext context.Context
		env         voldriver.Env
		err         error

		fakeInvoker *voldriverfakes.FakeInvoker

		subject nfsdriver.Mounter

		opts map[string]interface{}
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("nfs-mounter")
		testContext = context.TODO()
		env = driverhttp.NewHttpDriverEnv(logger, testContext)
		opts = map[string]interface{}{}

		fakeInvoker = &voldriverfakes.FakeInvoker{}

		subject = nfsv3driver.NewNfsV3Mounter(fakeInvoker, nfsv3driver.NewNfsV3Config(
			[]string{"uid,gid", ""},
			[]string{"sloppy_mount,fusenfs_uid,fusenfs_gid,multithread,default_permissions", "sloppy_mount:true"},
		))
	})

	Context("#Mount", func() {
		Context("when mount succeeds", func() {
			BeforeEach(func() {
				fakeInvoker.InvokeReturns(nil, nil)
				err = subject.Mount(env, "source", "target", opts)
			})

			It("should return without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should use the passed in variables", func() {
				_, cmd, args := fakeInvoker.InvokeArgsForCall(0)
				Expect(cmd).To(Equal("fuse-nfs"))
				testMountOptions(args, []string{
					"-n", "source", "-m", "target",
				})
				Expect(strings.Join(args, " ")).To(ContainSubstring("-n source"))
				Expect(strings.Join(args, " ")).To(ContainSubstring("-m target"))
			})
		})

		Context("when mount errors", func() {
			BeforeEach(func() {
				fakeInvoker.InvokeReturns([]byte("error"), fmt.Errorf("error"))
				err = subject.Mount(env, "source", "target", opts)
			})

			It("should return without error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when mount is cancelled", func() {
			// TODO: when we pick up the lager.Context
		})
	})

	Context("#Unmount", func() {
		Context("when mount succeeds", func() {

			BeforeEach(func() {
				fakeInvoker.InvokeReturns(nil, nil)

				err = subject.Unmount(env, "target")
			})

			It("should return without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should use the passed in variables", func() {
				_, cmd, args := fakeInvoker.InvokeArgsForCall(0)
				Expect(cmd).To(Equal("fusermount"))
				//Expect(args[1]).To(Equal("target"))

				expected := []string{"-u", "target"}
				Expect(args).To(Equal(expected))
				Expect(strings.Join(args, " ")).To(ContainSubstring("-u target"))
			})
		})

		Context("when unmount fails", func() {
			BeforeEach(func() {
				fakeInvoker.InvokeReturns([]byte("error"), fmt.Errorf("error"))

				err = subject.Unmount(env, "target")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Context("#Check", func() {

		var (
			success bool
		)

		Context("when check succeeds", func() {
			BeforeEach(func() {
				success = subject.Check(env, "target", "source")
			})
			It("uses correct context", func() {
				env, _, _ := fakeInvoker.InvokeArgsForCall(0)
				Expect(fmt.Sprintf("%#v", env.Context())).To(ContainSubstring("timerCtx"))
			})
			It("reports valid mountpoint", func() {
				Expect(success).To(BeTrue())
			})
		})
		Context("when check fails", func() {
			BeforeEach(func() {
				fakeInvoker.InvokeReturns([]byte("error"), fmt.Errorf("error"))
				success = subject.Check(env, "target", "source")
			})
			It("reports invalid mountpoint", func() {
				Expect(success).To(BeFalse())
			})
		})
	})

	Context("#Mount_opts", func() {
		Context("when mount succeeds with sloppy mount", func() {
			BeforeEach(func() {
				fakeInvoker.InvokeReturns(nil, nil)

				opts["default_permissions"] 	= true
				opts["multithread"] 		= "false"
				opts["fusenfs_uid"] 		= "1004"
				opts["fusenfs_gid"] 		= 1004
				opts["sloppy_mount"] 		= "true"
				opts["no_exists_opts"] 		= "example"

				err = subject.Mount(env, "source", "target", opts)
			})

			It("should return without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should use the passed in variables", func() {
				_, cmd, args := fakeInvoker.InvokeArgsForCall(0)
				Expect(cmd).To(Equal("fuse-nfs"))
				testMountOptions(args, []string{
					"-n", "source", "-m", "target",
					"--default_permissions", "--fusenfs_uid=1004", "--fusenfs_gid=1004",
				})
				Expect(strings.Join(args, " ")).To(ContainSubstring("-n source"))
				Expect(strings.Join(args, " ")).To(ContainSubstring("-m target"))
			})
		})

		Context("when mount errors without sloppy mount", func() {
			BeforeEach(func() {
				fakeInvoker.InvokeReturns(nil, nil)

				opts["default_permissions"] 	= true
				opts["multithread"] 		= "false"
				opts["fusenfs_uid"] 		= 1004
				opts["fusenfs_gid"] 		= "1004"
				opts["no_exists_opts"] 		= "example"

				err = subject.Mount(env, "source", "target", opts)
			})

			It("should return without error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when mount errors", func() {
			BeforeEach(func() {
				fakeInvoker.InvokeReturns([]byte("error"), fmt.Errorf("error"))

				err = subject.Mount(env, "source", "target", opts)
			})

			It("should return without error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Context("when mount is cancelled", func() {
			// TODO: when we pick up the lager.Context
		})
	})

})

func testMountOptions(args []string, expected []string) {
	Expect(len(args)).To(Equal(len(expected)))

	for _,p := range args {
		Expect(inArray(p, expected)).To(BeTrue())
	}

	for _,p := range expected {
		Expect(inArray(p, args)).To(BeTrue())
	}
}

func inArray (search string, slice []string) bool {
	for _,v := range slice {
		if v == search {
			return true
		}
	}

	return false
}