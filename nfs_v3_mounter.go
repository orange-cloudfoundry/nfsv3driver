package nfsv3driver

import (
	"context"
	"errors"
	"fmt"
	"time"


	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/nfsdriver"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"code.cloudfoundry.org/voldriver/invoker"

	"strings"
	"io/ioutil"
	"gopkg.in/yaml.v2"
	"os"
	"strconv"
)

type nfsV3Mounter struct {
	invoker invoker.Invoker
}

type Config struct {
	sourceOptions map[string]string
	mountOptions map[string]string

	sloppyMount bool
}

func NewNfsV3Mounter(invoker invoker.Invoker) nfsdriver.Mounter {
	return &nfsV3Mounter{invoker}
}

func (m *nfsV3Mounter) Mount(env voldriver.Env, source string, target string, opts map[string]interface{}) error {
	logger := env.Logger().Session("fuse-nfs-mount")
	logger.Info("start")
	defer logger.Info("end")

	myCnf := new(Config)

	if err := myCnf.getConf("config.yml", logger); err != nil {
		return err;
	}

	if err := myCnf.filterMount(opts, logger); err != nil {
		return err;
	}

	if err := myCnf.makeShare(&source, logger); err != nil {
		return err;
	}

	logger.Debug("parse-mount", lager.Data{"source": source, "target": target, "options": opts})

	mountParams := append([]string{
		"-n", source,
		"-m", target,
	}, myCnf.getMountOptions(logger)...)

	if _,ok := myCnf.mountOptions["a"]; !ok {
		if len(mountParams) <= 4 {
			mountParams = append(mountParams, "-a")
		}
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

func (m *Config) getConf(configPath string, logger lager.Logger) error {

	type ConfigYaml  struct {
		SrcString string `yaml:"source_params"`
		MntString string `yaml:"mount_params"`
	}

	file, err := os.Open(configPath)
	if err != nil {
		logger.Fatal("nfsv3driver-config", err, lager.Data{"file": configPath})
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		logger.Fatal("nfsv3driver-config", err, lager.Data{"file": configPath})
	}

	var configYaml ConfigYaml

	err = yaml.Unmarshal(data, &configYaml)
	if err != nil {
		logger.Fatal("nfsv3driver-config", err, lager.Data{"file": configPath})
	}

	m.mountOptions = m.parseConfig(strings.Split(configYaml.MntString, ","))
	m.sourceOptions = m.parseConfig(strings.Split(configYaml.SrcString, ","))
	m.sloppyMount = m.initSloppyMount()

	logger.Debug("nfsv3driver-config-loaded", lager.Data{"sloppyMount": m.sloppyMount, "sourceOptions": m.sourceOptions, "mountOptions": m.mountOptions})

	return nil
}

func (m *Config) parseConfig(listEntry []string) map[string]string {

	result := map[string]string{}

	for _,opt := range listEntry {

		key := strings.SplitN(opt, ":", 2)

		if len(key[0]) < 1 {
			continue
		}

		if len(key[1]) < 1 {
			result[key[0]] = ""
		} else {
			result[key[0]] = key[1]
		}
	}

	return result
}

func (m *Config) initSloppyMount() bool {
	if _, ok := m.mountOptions["sloppy_mount"]; ok {

		if val,err := strconv.ParseBool(m.mountOptions["sloppy_mount"]); err == nil {
			return val
		}
	}

	return false
}

func (m *Config) filterSource (entryList []string, logger lager.Logger) error {

	var errorList []string

	for _, p := range entryList {

		op := strings.SplitN(p, "=", 2)

		if len (op) < 2 || len(op[1]) < 1 || op[1] == "" {
			continue
		}

		if _,ok := m.sourceOptions[op[0]]; !ok {
			errorList = append(errorList, op[0]);
			continue
		}

		m.sourceOptions[op[0]] = op[1]
	}

	logger.Debug("nfsv3driver-source-opts-parsed", lager.Data{"config": m.sourceOptions, "error": errorList})

	if len(errorList) > 0 && !m.sloppyMount {
		err := errors.New("Incompatibles source options !")
		logger.Error("nfsv3driver-source-opts", err, lager.Data{"errors": errorList})
		return err
	}

	if len(errorList) > 0 {
		logger.Info("nfsv3driver-source-opts", lager.Data{"imcopatibles-opts": errorList})
	}

	return nil
}

func (m *Config) filterMount (entryList map[string]interface{}, logger lager.Logger) error {

	var errorList []string

	cleanEntry := m.uniformEntry(entryList, logger)

	for k, v := range cleanEntry {

		if v == ""  {
			continue
		}

		if _,ok := m.mountOptions[k]; !ok {
			errorList = append(errorList, k);
			continue
		}

		if val, err := strconv.ParseBool(v); err == nil {
			if val == true && k == "sloppy_mount" {
				m.sloppyMount = true
				continue
			}
		}

		m.mountOptions[k] = v
	}

	logger.Debug("nfsv3driver-mount-opts-parsed", lager.Data{"config": m.mountOptions, "error": errorList})

	if len(errorList) > 0 && !m.sloppyMount {
		err := errors.New("Incompatibles source options !")
		logger.Error("nfsv3driver-mount-opts", err, lager.Data{"errors": errorList})
		return err
	}

	if len(errorList) > 0 {
		logger.Info("nfsv3driver-mount-opts", lager.Data{"imcompatibles-opts": errorList})
	}

	return nil
}

func (m *Config) makeShare(url *string, logger lager.Logger) error {

	srcPart := strings.SplitN(*url, "?", 2)

	if len(srcPart) == 1 {
		srcPart = append(srcPart, "")
	}

	if err := m.filterSource(strings.Split(srcPart[1], "&"), logger); err != nil {
		return err;
	}

	paramsList := []string{}

	for k,v := range m.sourceOptions  {
		if v == "" {
			continue
		}

		if val, err := strconv.ParseBool(v); err == nil {
			if val == true {
				paramsList = append(paramsList, fmt.Sprintf("%s=1", k))
			} else {
				paramsList = append(paramsList, fmt.Sprintf("%s=0", k))
			}
			continue
		}

		if val, err := strconv.ParseInt(v, 10, 16); err == nil {
			paramsList = append(paramsList, fmt.Sprintf("%s=%d", k, val))
			continue
		}

		paramsList = append(paramsList, fmt.Sprintf("%s=%s", k, v))
	}

	srcPart[1] = strings.Join(paramsList, "&")

	if len(srcPart[1]) < 1 {
		*url = srcPart[0]
	} else {
		*url = strings.Join(srcPart, "?")
	}

	return nil
}

func (m *Config) getMountOptions(logger lager.Logger) []string {

	result := []string{}
	var pid string

	for k,v := range m.mountOptions  {

		if k == "sloppy_mount" || v == "" {
			continue
		}

		if len(k) == 1 {
			pid = "-"
		} else {
			pid = "--"
		}

		if val, err := strconv.ParseBool(v); err == nil {
			if (val == true) {
				result = append(result, fmt.Sprintf("%s%s", pid, k))
			}
			continue
		}

		if val, err := strconv.ParseInt(v, 10, 16); err == nil {
			result = append(result, fmt.Sprintf("%s%s=%d", pid, k, val))
			continue
		}

		result = append(result, fmt.Sprintf("%s%s=%s", pid, k, v))
	}

	return result
}

func (m *Config) uniformEntry (entryList map[string]interface{}, logger lager.Logger) map[string]string {

	result := map[string]string{}

	for k, v := range entryList {

		var value interface{}

		switch v.(type) {
		case int:
			value = strconv.FormatInt(int64(v.(int)), 10)
		case string:
			value = v.(string)
		case bool:
			value = strconv.FormatBool(v.(bool))
		default:
			value = ""
		}

		result[k] = value.(string)
	}

	return result
}