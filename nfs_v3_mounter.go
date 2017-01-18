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
}

func NewNfsV3Mounter(invoker invoker.Invoker) nfsdriver.Mounter {
	return &nfsV3Mounter{invoker}
}

func (m *nfsV3Mounter) Mount(env voldriver.Env, source string, target string, opts map[string]interface{}) error {
	logger := env.Logger().Session("fuse-nfs-mount")
	logger.Info("start")
	defer logger.Info("end")

	logger.Debug("parse-mount", lager.Data{"source": source, "target": target})

	mountParams := []string{
		"-a",
	}

	params := strings.Split(source, '?')
	var source []string

	for _, p := range strings.Split(params[1], '&') {

		if strings.Contains(p, "uid=") && !strings.Contains(p, "nfs_uid=") {
			source = append(source, p)
			continue
		}

		if strings.Contains(p, "gid=") && !strings.Contains(p, "nfs_gid=") {
			source = append(source, p)
			continue
		}

		mountParams = append(mountParams, "--" + p)
	}

	share := fmt.Sprintf("%s?%s", params[0], strings.Join(source, "&"))

	logger.Debug("exec-mount", lager.Data{"source": share, "target": target, "options": strings.Join(mountParams, ",")})

	mountParams = append(mountParams, "-n")
	mountParams = append(mountParams, share)

	mountParams = append(mountParams, "-m")
	mountParams = append(mountParams, target)

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
