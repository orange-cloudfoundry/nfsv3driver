package nfsv3driver

import (
	"context"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/nfsdriver"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"code.cloudfoundry.org/voldriver/invoker"

	"strings"
)

type nfsV3Mounter struct {
	invoker invoker.Invoker
	config  Config
}

func NewNfsV3Mounter(invoker invoker.Invoker, config **Config) nfsdriver.Mounter {
	return &nfsV3Mounter{invoker, **config}
}

func (m *nfsV3Mounter) Mount(env voldriver.Env, source string, target string, opts map[string]interface{}) error {
	logger := env.Logger().Session("fuse-nfs-mount")
	logger.Info("start")
	defer logger.Info("end")

	if err := m.config.setEntries(source, opts, []string{
		"share", "mount", "kerberosPrincipal", "kerberosKeytab", "readonly",
	}); err != nil {
		logger.Debug("parse-entries", lager.Data{
			"given_source":   source,
			"given_target":   target,
			"given_options":  opts,
			"source.allowed": m.config.source.allowed,
			"source.options": m.config.source.options,
			"source.forced":  m.config.source.forced,
			"mount.allowed":  m.config.mount.allowed,
			"mount.options":  m.config.mount.options,
			"mount.forced":   m.config.mount.forced,
			"sloppy_mount":   m.config.sloppyMount,
		})
		return err
	}

	mountOptions := m.config.getMount()
	sourceParsed := m.config.getShare(source)

	logger.Debug("parse-mount", lager.Data{
		"given_source":   source,
		"given_target":   target,
		"given_options":  opts,
		"source.allowed": m.config.source.allowed,
		"source.options": m.config.source.options,
		"source.forced":  m.config.source.forced,
		"mount.allowed":  m.config.mount.allowed,
		"mount.options":  m.config.mount.options,
		"mount.forced":   m.config.mount.forced,
		"sloppy_mount":   m.config.sloppyMount,
		"mountOptions":   mountOptions,
		"sourceParsed":   sourceParsed,
	})

	mountParams := append([]string{
		"-n", sourceParsed,
		"-m", target,
	}, mountOptions...)

	logger.Debug("exec-mount", lager.Data{"params": strings.Join(mountParams, ",")})
	_, err := m.invoker.Invoke(env, "fuse-nfs", mountParams)

	return err
}

func (m *nfsV3Mounter) Unmount(env voldriver.Env, target string) error {
	_, err := m.invoker.Invoke(env, "fusermount", []string{"-u", target})
	return err
}

func (m *nfsV3Mounter) Check(env voldriver.Env, name, mountPoint string) bool {
	ctx, _ := context.WithDeadline(context.TODO(), time.Now().Add(time.Second*5))
	env = driverhttp.EnvWithContext(ctx, env)
	_, err := m.invoker.Invoke(env, "mountpoint", []string{"-q", mountPoint})

	if err != nil {
		// Note: Created volumes (with no mounts) will be removed
		//       since VolumeInfo.Mountpoint will be an empty string
		env.Logger().Info(fmt.Sprintf("unable to verify volume %s (%s)", name, err.Error()))
		return false
	}
	return true
}
