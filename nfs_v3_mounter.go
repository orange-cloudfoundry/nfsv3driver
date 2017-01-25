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
	"reflect"
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

	logger.Debug("parse-mount", lager.Data{"source": source, "target": target, "options": opts})

	mountParams := []string{
		"-n", source,
		"-m", target,
	}

	whitelist := []string{
		// Fuse_NFS Options
		"fusenfs_allow_other_own_ids",
		"fusenfs_uid",
		"fusenfs_gid",

		// libNFS options
		// options for libnfs need to be
		// pass direction url form for share

		// Fuse Option (see man mount.fuse)
		"default_permissions",
		"multithread",
		"allow_other",
		"allow_root",
		"umask",
		"direct_io",
		"kernel_cache",
		"auto_cache",
		"entry_timeout",
		"negative_timeout",
		"attr_timeout",
		"ac_attr_timeout",
		"large_read",
		"hard_remove",
		"fsname",
		"subtype",
		"blkdev",
		"intr",
		"mount_max",
		"max_read",
		"max_readahead",
		"async_read",
		"sync_read",
		"nonempty",
		"intr_signal",
		"use_ino",
		"readdir_ino",
		"debug",
	}

	for k, v := range opts {

		if !in_array(k, whitelist) {
			continue
		}

		logger.Debug("Parse one Options ", lager.Data{"Key": k, "value": v})

		val, err := v.(bool)

		if err {
			if val {
				mountParams = append(mountParams, fmt.Sprintf("--%s", k))
			}
		} else {
			mountParams = append(mountParams, fmt.Sprintf("--%s=%v", k, v))
		}
	}

	if len(mountParams) == 4 {
		mountParams = append(mountParams, "-a");
	}

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

func in_array(val interface{}, array interface{}) (exists bool) {
	exists = false

	switch reflect.TypeOf(array).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(array)

		for i := 0; i < s.Len(); i++ {
			if reflect.DeepEqual(val, s.Index(i).Interface()) == true {
				exists = true
				return
			}
		}
	}

	return
}