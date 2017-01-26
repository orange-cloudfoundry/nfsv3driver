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
	"path/filepath"
	"io/ioutil"
	"gopkg.in/yaml.v2"
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

	myCnf, ok := readConfigNFS(logger);

	if ok != nil {
		myCnf = getDefaultConf();
	}

	var errorParams []string
	sloppyMount := false

	for k, v := range opts {

		val, err := v.(bool)

		if k == "sloppy_mount" || k == "-s" {
			sloppyMount = val && err
			continue
		}

		if !stringInSlice(k, myCnf) {
			errorParams = append(errorParams, k)
			continue
		}

		logger.Debug("Whitelisted options", lager.Data{"Options": k})

		if err {
			if val {
				mountParams = append(mountParams, fmt.Sprintf("--%s", k))
			}

		} else {
			mountParams = append(mountParams, fmt.Sprintf("--%s=%v", k, v))
		}
	}

	if sloppyMount != true && len(errorParams) > 0 {
		err := fmt.Errorf("Incompatibles mount options without sloppy mount mode !")
		logger.Error("mount-opts", err, lager.Data{"errors": errorParams})
		return err
	}

	if len(errorParams) > 0 {
		logger.Info("mount-opts", lager.Data{"ignore": errorParams})
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

func stringInSlice(a string, list []string) bool {

	for _, b := range list {
		if b == a {
			return true
		}
	}

	return false
}

func readConfigNFS(logger lager.Logger) ([]string, error) {

	type Config struct {
		mountOptions []string
	}

	filename, _ := filepath.Abs("manifest.yml")
	yamlFile, err := ioutil.ReadFile(filename)

	if err != nil {
		logger.Error("read NFS Config", err, lager.Data{"file": filename, "yaml": yamlFile})
		return nil, err
	}

	var config Config

	err = yaml.Unmarshal(yamlFile, &config)

	if err != nil {
		logger.Error("Parse NFS Config", err, lager.Data{"config": config})
		return nil, err
	}

	return config.mountOptions, nil
}

func getDefaultConf() []string {

	return []string{
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
}