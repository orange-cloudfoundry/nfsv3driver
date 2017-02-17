package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"

	cf_http "code.cloudfoundry.org/cfhttp"
	cf_lager "code.cloudfoundry.org/cflager"
	cf_debug_server "code.cloudfoundry.org/debugserver"

	"code.cloudfoundry.org/goshims/filepathshim"
	"code.cloudfoundry.org/goshims/ioutilshim"
	"code.cloudfoundry.org/goshims/osshim"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/nfsdriver"
	"code.cloudfoundry.org/nfsv3driver"
	"code.cloudfoundry.org/voldriver"
	"code.cloudfoundry.org/voldriver/driverhttp"
	"code.cloudfoundry.org/voldriver/invoker"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/http_server"
	"github.com/tedsuo/ifrit/sigmon"
)

var atAddress = flag.String(
	"listenAddr",
	"0.0.0.0:7589",
	"host:port to serve volume management functions",
)

var driversPath = flag.String(
	"driversPath",
	"",
	"Path to directory where drivers are installed",
)

var transport = flag.String(
	"transport",
	"tcp",
	"Transport protocol to transmit HTTP over",
)

var mountDir = flag.String(
	"mountDir",
	"/tmp/volumes",
	"Path to directory where fake volumes are created",
)

var requireSSL = flag.Bool(
	"requireSSL",
	false,
	"whether the fake driver should require ssl-secured communication",
)

var caFile = flag.String(
	"caFile",
	"",
	"the certificate authority public key file to use with ssl authentication",
)

var certFile = flag.String(
	"certFile",
	"",
	"the public key file to use with ssl authentication",
)

var keyFile = flag.String(
	"keyFile",
	"",
	"the private key file to use with ssl authentication",
)
var clientCertFile = flag.String(
	"clientCertFile",
	"",
	"the public key file to use with client ssl authentication",
)

var clientKeyFile = flag.String(
	"clientKeyFile",
	"",
	"the private key file to use with client ssl authentication",
)

var insecureSkipVerify = flag.Bool(
	"insecureSkipVerify",
	false,
	"whether SSL communication should skip verification of server IP addresses in the certificate",
)

var sourceFlagAllowed = flag.String(
	"allowed-in-source",
	"",
	"This is a comma separted list of parameters allowed to be send in share url. Each of this parameters can be specify by brokers",
)

var sourceFlagDefault = flag.String(
	"default-in-source",
	"",
	"This is a comma separted list of like params:value. This list specify default value of parameters. If parameters has default value and is not in allowed list, this default value become a forced value who's cannot be override",
)

var mountFlagAllowed = flag.String(
	"allowed-in-mount",
	"",
	"This is a comma separted list of parameters allowed to be send in extra config. Each of this parameters can be specify by brokers",
)

var mountFlagDefault = flag.String(
	"default-in-mount",
	"",
	"This is a comma separted list of like params:value. This list specify default value of parameters. If parameters has default value and is not in allowed list, this default value become a forced value who's cannot be override",
)

const fsType = "nfs4"
const mountOptions = "vers=4.0,rsize=1048576,wsize=1048576,hard,intr,timeo=600,retrans=2,actimeo=0"

func main() {
	parseCommandLine()

	var localDriverServer ifrit.Runner

	logger, logTap := newLogger()
	logger.Info("start")
	defer logger.Info("end")

	source := nfsv3driver.NewNfsV3ConfigDetails()
	source.ReadConf(*sourceFlagAllowed, *sourceFlagDefault, []string{"uid", "gid"})

	mounts := nfsv3driver.NewNfsV3ConfigDetails()
	mounts.ReadConf(*mountFlagAllowed, *mountFlagDefault, []string{})

	mounter := nfsv3driver.NewNfsV3Mounter(
		invoker.NewRealInvoker(),
		nfsv3driver.NewNfsV3Config(source, mounts),
	)

	client := nfsdriver.NewNfsDriver(
		logger,
		&osshim.OsShim{},
		&filepathshim.FilepathShim{},
		&ioutilshim.IoutilShim{},
		*mountDir,
		mounter,
	)

	if *transport == "tcp" {
		localDriverServer = createNfsDriverServer(logger, client, *atAddress, *driversPath, false)
	} else if *transport == "tcp-json" {
		localDriverServer = createNfsDriverServer(logger, client, *atAddress, *driversPath, true)
	} else {
		localDriverServer = createNfsDriverUnixServer(logger, client, *atAddress)
	}

	servers := grouper.Members{
		{"localdriver-server", localDriverServer},
	}

	if dbgAddr := cf_debug_server.DebugAddress(flag.CommandLine); dbgAddr != "" {
		servers = append(grouper.Members{
			{"debug-server", cf_debug_server.Runner(dbgAddr, logTap)},
		}, servers...)
	}

	process := ifrit.Invoke(processRunnerFor(servers))
	logger.Info("started")

	untilTerminated(logger, process)
}

func exitOnFailure(logger lager.Logger, err error) {
	if err != nil {
		logger.Error("fatal-err..aborting", err)
		panic(err.Error())
	}
}

func untilTerminated(logger lager.Logger, process ifrit.Process) {
	err := <-process.Wait()
	exitOnFailure(logger, err)
}

func processRunnerFor(servers grouper.Members) ifrit.Runner {
	return sigmon.New(grouper.NewOrdered(os.Interrupt, servers))
}

func createNfsDriverServer(logger lager.Logger, client voldriver.Driver, atAddress, driversPath string, jsonSpec bool) ifrit.Runner {
	advertisedUrl := "http://" + atAddress
	logger.Info("writing-spec-file", lager.Data{"location": driversPath, "name": "nfsv3driver", "address": advertisedUrl})
	if jsonSpec {
		driverJsonSpec := voldriver.DriverSpec{Name: "nfsv3driver", Address: advertisedUrl}

		if *requireSSL {
			absCaFile, err := filepath.Abs(*caFile)
			exitOnFailure(logger, err)
			absClientCertFile, err := filepath.Abs(*clientCertFile)
			exitOnFailure(logger, err)
			absClientKeyFile, err := filepath.Abs(*clientKeyFile)
			exitOnFailure(logger, err)
			driverJsonSpec.TLSConfig = &voldriver.TLSConfig{InsecureSkipVerify: *insecureSkipVerify, CAFile: absCaFile, CertFile: absClientCertFile, KeyFile: absClientKeyFile}
			driverJsonSpec.Address = "https://" + atAddress
		}

		jsonBytes, err := json.Marshal(driverJsonSpec)

		exitOnFailure(logger, err)
		err = voldriver.WriteDriverSpec(logger, driversPath, "nfsv3driver", "json", jsonBytes)
		exitOnFailure(logger, err)
	} else {
		err := voldriver.WriteDriverSpec(logger, driversPath, "nfsv3driver", "spec", []byte(advertisedUrl))
		exitOnFailure(logger, err)
	}

	handler, err := driverhttp.NewHandler(logger, client)
	exitOnFailure(logger, err)

	var server ifrit.Runner
	if *requireSSL {
		tlsConfig, err := cf_http.NewTLSConfig(*certFile, *keyFile, *caFile)
		if err != nil {
			logger.Fatal("tls-configuration-failed", err)
		}
		server = http_server.NewTLSServer(atAddress, handler, tlsConfig)
	} else {
		server = http_server.New(atAddress, handler)
	}

	return server
}

func createNfsDriverUnixServer(logger lager.Logger, client voldriver.Driver, atAddress string) ifrit.Runner {
	handler, err := driverhttp.NewHandler(logger, client)
	exitOnFailure(logger, err)
	return http_server.NewUnixServer(atAddress, handler)
}

func newLogger() (lager.Logger, *lager.ReconfigurableSink) {
	logger, reconfigurableSink := cf_lager.New("nfs-driver-server")
	return logger, reconfigurableSink
}

func parseCommandLine() {
	cf_lager.AddFlags(flag.CommandLine)
	cf_debug_server.AddFlags(flag.CommandLine)
	flag.Parse()
}
