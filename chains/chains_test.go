package chains

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eris-ltd/eris-cli/config"
	"github.com/eris-ltd/eris-cli/definitions"
	"github.com/eris-ltd/eris-cli/log"
	"github.com/eris-ltd/eris-cli/services"
	"github.com/eris-ltd/eris-cli/testutil"
	"github.com/eris-ltd/eris-cli/util"

	"github.com/eris-ltd/common/go/common"
)

var (
	erisDir   = filepath.Join(os.TempDir(), "eris")
	chainName = "test-chain"
)

func TestMain(m *testing.M) {
	log.SetLevel(log.ErrorLevel)
	// log.SetLevel(log.InfoLevel)
	// log.SetLevel(log.DebugLevel)

	testutil.IfExit(testutil.Init(testutil.Pull{
		Images: []string{"data", "cm", "db", "keys", "ipfs"},
	}))

	exitCode := m.Run()

	testutil.IfExit(testutil.TearDown())

	os.Exit(exitCode)
}

func TestStartChain(t *testing.T) {
	defer testutil.RemoveAllContainers()

	create(t, chainName)

	if !util.Running(definitions.TypeChain, chainName) {
		t.Fatalf("expecting chain running")
	}
	if !util.Exists(definitions.TypeData, chainName) {
		t.Fatalf("expecting dependent data container exists")
	}

	kill(t, chainName)
	if util.Running(definitions.TypeChain, chainName) {
		t.Fatalf("expecting chain doesn't run")
	}
	if util.Exists(definitions.TypeData, chainName) {
		t.Fatalf("expecting data container doesn't exist")
	}
}

func TestRestartChain(t *testing.T) {
	defer testutil.RemoveAllContainers()

	// make a chain
	create(t, chainName)
	if !util.Running(definitions.TypeChain, chainName) {
		t.Fatalf("expecting chain running")
	}
	if !util.Exists(definitions.TypeData, chainName) {
		t.Fatalf("expecting data container exists")
	}

	// stop it
	stop(t, chainName)
	if util.Running(definitions.TypeChain, chainName) {
		t.Fatalf("expecting chain doesn't run")
	}
	if !util.Exists(definitions.TypeData, chainName) {
		t.Fatalf("expecting data container exists")
	}

	// start it back up again
	start(t, chainName)
	if !util.Running(definitions.TypeChain, chainName) {
		t.Fatalf("expecting chain running")
	}
	if !util.Exists(definitions.TypeData, chainName) {
		t.Fatalf("expecting data container exists")
	}

	// kill it
	kill(t, chainName)
	if util.Running(definitions.TypeChain, chainName) {
		t.Fatalf("expecting chain doesn't run")
	}
	if util.Exists(definitions.TypeData, chainName) {
		t.Fatalf("expecting data container doesn't exist")
	}
}

func TestExecChain(t *testing.T) {
	defer testutil.RemoveAllContainers()

	create(t, chainName)
	defer kill(t, chainName)

	do := definitions.NowDo()
	do.Name = chainName
	do.Operations.Args = []string{"ls", common.ErisContainerRoot}
	buf, err := ExecChain(do)
	if err != nil {
		t.Fatalf("expected chain to execute, got %v", err)
	}

	if dir := "chains"; !strings.Contains(buf.String(), dir) {
		t.Fatalf("expected to find %q dir in eris root", dir)
	}
}

func TestExecChainBadCommandLine(t *testing.T) {
	defer testutil.RemoveAllContainers()

	create(t, chainName)
	defer kill(t, chainName)

	do := definitions.NowDo()
	do.Name = chainName
	do.Operations.Args = strings.Fields("bad command line")
	if _, err := ExecChain(do); err == nil {
		t.Fatalf("expected chain to fail")
	}
}

func TestCatChainContainerConfig(t *testing.T) {
	defer testutil.RemoveAllContainers()

	buf := new(bytes.Buffer)
	config.Global.Writer = buf

	const chain = "test-cat-cont-config"

	create(t, chain)
	defer kill(t, chain)

	do := definitions.NowDo()
	do.Name = chain
	do.Type = "config"
	if err := CatChain(do); err != nil {
		t.Fatalf("expected getting a local config to succeed, got %v", err)
	}

	if !strings.Contains(buf.String(), "moniker") { // [zr] we should test a few more things in the config.toml generated by ecm?
		t.Fatalf("expected the config file to contain an expected string, got %v", buf.String())
	}
}

func TestCatChainContainerGenesis(t *testing.T) {
	defer testutil.RemoveAllContainers()

	buf := new(bytes.Buffer)
	config.Global.Writer = buf

	create(t, chainName)
	defer kill(t, chainName)

	do := definitions.NowDo()
	do.Name = chainName
	do.Type = "genesis"
	if err := CatChain(do); err != nil {
		t.Fatalf("expected getting a local config to succeed, got %v", err)
	}

	if !strings.Contains(buf.String(), "accounts") || !strings.Contains(buf.String(), "validators") {
		t.Fatalf("expected the genesis file to contain expected strings, got %v", buf.String())
	}
}

func TestChainsNewDirGenesis(t *testing.T) {
	defer testutil.RemoveAllContainers()

	const chain = "test-dir-gen"
	create(t, chain)
	defer kill(t, chain)

	args := []string{"cat", fmt.Sprintf("/home/eris/.eris/chains/%s/genesis.json", chain)}
	if out := exec(t, chain, args); !strings.Contains(out, chain) {
		t.Fatalf("expected chain_id to be equal to chain name in genesis file, got %v", out)
	}
}

func TestChainsNewConfig(t *testing.T) {
	defer testutil.RemoveAllContainers()

	const chain = "test-config-new"
	create(t, chain)
	defer kill(t, chain)

	args := []string{"cat", fmt.Sprintf("/home/eris/.eris/chains/%s/config.toml", chain)}
	if out := exec(t, chain, args); !strings.Contains(out, "moniker") {
		t.Fatalf("expected the config file to contain an expected string, got %v", out)
	}
}

// chains new should import the priv_validator.json (available in mint form)
// into eris-keys (available in eris form) so it can be used by the rest
// of the platform
func TestChainsNewKeysImported(t *testing.T) {
	defer testutil.RemoveAllContainers()

	const chain = "test-config-keys"
	create(t, chain)
	defer kill(t, chain)

	if !util.Running(definitions.TypeChain, chain) {
		t.Fatalf("expecting chain running")
	}

	keysOut, err := services.ExecHandler("keys", []string{"ls", "/home/eris/.eris/keys/data"})
	if err != nil {
		t.Fatalf("expecting to list keys, got %v", err)
	}

	keysOutString0 := strings.Fields(strings.TrimSpace(keysOut.String()))[0]

	args := []string{"cat", fmt.Sprintf("/home/eris/.eris/keys/data/%s/%s", keysOutString0, keysOutString0)}

	keysOut1, err := services.ExecHandler("keys", args)
	if err != nil {
		t.Fatalf("expecting to cat keys, got %v", err)
	}

	keysOutString1 := strings.Fields(strings.TrimSpace(keysOut1.String()))[0]

	if !strings.Contains(keysOutString1, keysOutString0) { // keysOutString0 is the substring (addr only)
		t.Fatalf("keys do not match, key0: %v, key1: %v", keysOutString0, keysOutString1)
	}
}

func TestLogsChain(t *testing.T) {
	defer testutil.RemoveAllContainers()

	create(t, chainName)
	defer kill(t, chainName)

	do := definitions.NowDo()
	do.Name = chainName
	do.Follow = false
	do.Tail = "all"
	if err := LogsChain(do); err != nil {
		t.Fatalf("failed to fetch container logs")
	}
}

func TestInspectChain(t *testing.T) {
	defer testutil.RemoveAllContainers()

	create(t, chainName)
	defer kill(t, chainName)

	do := definitions.NowDo()
	do.Name = chainName
	do.Operations.Args = []string{"name"}
	if err := InspectChain(do); err != nil {
		t.Fatalf("expected chain to be inspected, got %v", err)
	}
}

func TestRmChain(t *testing.T) {
	defer testutil.RemoveAllContainers()

	create(t, chainName)

	do := definitions.NowDo()
	do.Operations.Args, do.Rm, do.RmD = []string{"keys"}, true, true
	if err := services.KillService(do); err != nil {
		t.Fatalf("expected service to be stopped, got %v", err)
	}

	kill(t, chainName) // implements RemoveChain
	if util.Exists(definitions.TypeChain, chainName) {
		t.Fatalf("expecting chain not running")
	}
}

func TestServiceLinkNoChain(t *testing.T) {
	defer testutil.RemoveAllContainers()

	if err := testutil.FakeServiceDefinition("fake", `
chain = "$chain:fake"

[service]
name = "fake"
image = "`+path.Join(config.Global.DefaultRegistry, config.Global.ImageIPFS)+`"
data_container = true
`); err != nil {
		t.Fatalf("can't create a fake service definition: %v", err)
	}

	do := definitions.NowDo()
	do.Operations.Args = []string{"fake"}
	if err := services.StartService(do); err == nil {
		t.Fatalf("expect start service to fail, got nil")
	}
}

func TestServiceLinkBadChain(t *testing.T) {
	defer testutil.RemoveAllContainers()

	if err := testutil.FakeServiceDefinition("fake", `
chain = "$chain:fake"

[service]
name = "fake"
image = "`+path.Join(config.Global.DefaultRegistry, config.Global.ImageIPFS)+`"
`); err != nil {
		t.Fatalf("can't create a fake service definition: %v", err)
	}

	do := definitions.NowDo()
	do.Operations.Args = []string{"fake"}
	do.ChainName = "non-existent-chain"
	if err := services.StartService(do); err == nil {
		t.Fatalf("expect start service to fail, got nil")
	}
}

func TestServiceLinkBadChainWithoutChainInDefinition(t *testing.T) {
	defer testutil.RemoveAllContainers()

	create(t, chainName)
	defer kill(t, chainName)

	if err := testutil.FakeServiceDefinition("fake", `
[service]
name = "fake"
image = "`+path.Join(config.Global.DefaultRegistry, config.Global.ImageIPFS)+`"
`); err != nil {
		t.Fatalf("can't create a fake service definition: %v", err)
	}

	do := definitions.NowDo()
	do.Operations.Args = []string{"fake"}
	do.ChainName = "non-existent-chain"

	// [pv]: is this a bug? the service which doesn't have a
	// "chain" in its definition file doesn't care about linking at all.
	if err := services.StartService(do); err != nil {
		t.Fatalf("expect service to start, got %v", err)
	}

	if !util.Running(definitions.TypeService, "fake") {
		t.Fatalf("expecting fake service running")
	}
	if util.Exists(definitions.TypeData, "fake") {
		t.Fatalf("expecting fake data container doesn't exist")
	}
}

func TestServiceLink(t *testing.T) {
	defer testutil.RemoveAllContainers()

	const chain = "test-chain-link"
	create(t, chain)
	defer kill(t, chain)

	if err := testutil.FakeServiceDefinition("fake", `
chain = "$chain:fake"

[service]
name = "fake"
image = "`+path.Join(config.Global.DefaultRegistry, config.Global.ImageKeys)+`"
data_container = false
`); err != nil {
		t.Fatalf("can't create a fake service definition: %v", err)
	}

	if !util.Exists(definitions.TypeChain, chain) {
		t.Fatalf("expecting fake chain container")
	}
	if util.Running(definitions.TypeService, "fake") {
		t.Fatalf("expecting fake service running")
	}
	if util.Exists(definitions.TypeData, "fake") {
		t.Fatalf("expecting fake data container doesn't exist")
	}

	do := definitions.NowDo()
	do.Operations.Args = []string{"fake"}
	do.ChainName = chain
	if err := services.StartService(do); err != nil {
		t.Fatalf("expecting service to start, got %v", err)
	}

	if !util.Running(definitions.TypeService, "fake") {
		t.Fatalf("expecting fake service not running")
	}
	if util.Exists(definitions.TypeData, "fake") {
		t.Fatalf("expecting fake data container doesn't exist")
	}

	links := testutil.Links("fake", definitions.TypeService)
	if len(links) != 1 || !strings.Contains(links[0], "/fake") {
		t.Fatalf("expected service be linked to a test chain, got %v", links)
	}
}

func TestServiceLinkWithDataContainer(t *testing.T) {
	defer testutil.RemoveAllContainers()

	const chain = "test-chain-data-container"

	create(t, chain)
	defer kill(t, chain)

	if err := testutil.FakeServiceDefinition("fake", `
chain = "$chain:fake"

[service]
name = "fake"
image = "`+path.Join(config.Global.DefaultRegistry, config.Global.ImageIPFS)+`"
data_container = true
`); err != nil {
		t.Fatalf("can't create a fake service definition: %v", err)
	}

	if !util.Exists(definitions.TypeChain, chain) {
		t.Fatalf("expecting test chain container")
	}
	if util.Running(definitions.TypeService, "fake") {
		t.Fatalf("expecting fake service not running")
	}
	if util.Exists(definitions.TypeData, "fake") {
		t.Fatalf("expecting fake data container doesn't exist")
	}

	do := definitions.NowDo()
	do.Operations.Args = []string{"fake"}
	do.ChainName = chain
	if err := services.StartService(do); err != nil {
		t.Fatalf("expecting service to start, got %v", err)
	}

	if !util.Running(definitions.TypeService, "fake") {
		t.Fatalf("expecting fake service running")
	}
	if !util.Exists(definitions.TypeData, "fake") {
		t.Fatalf("expecting fake data container exists")
	}

	links := testutil.Links("fake", definitions.TypeService)
	if len(links) != 1 || !strings.Contains(links[0], "/fake") {
		t.Fatalf("expected service be linked to a test chain, got %v", links)
	}
}

func TestServiceLinkLiteral(t *testing.T) {
	defer testutil.RemoveAllContainers()

	const chain = "test-chain-literal"

	create(t, chain)
	defer kill(t, chain)

	if err := testutil.FakeServiceDefinition("fake", `
chain = "`+chain+`:fake"

[service]
name = "fake"
image = "`+path.Join(config.Global.DefaultRegistry, config.Global.ImageKeys)+`"
`); err != nil {
		t.Fatalf("can't create a fake service definition: %v", err)
	}

	if !util.Exists(definitions.TypeChain, chain) {
		t.Fatalf("expecting fake chain container")
	}
	if util.Running(definitions.TypeService, "fake") {
		t.Fatalf("expecting fake service not running")
	}
	if util.Exists(definitions.TypeData, "fake") {
		t.Fatalf("expecting fake data container doesn't exist")
	}

	do := definitions.NowDo()
	do.Operations.Args = []string{"fake"}
	do.ChainName = chain
	if err := services.StartService(do); err != nil {
		t.Fatalf("expecting service to start, got %v", err)
	}

	if !util.Running(definitions.TypeService, "fake") {
		t.Fatalf("expecting fake service running")
	}
	if util.Exists(definitions.TypeData, "fake") {
		t.Fatalf("expecting fake data container exists")
	}

	links := testutil.Links("fake", definitions.TypeService)
	if len(links) != 1 || !strings.Contains(links[0], "/fake") {
		t.Fatalf("expected service be linked to a test chain, got %v", links)
	}
}

func TestServiceLinkBadLiteral(t *testing.T) {
	defer testutil.RemoveAllContainers()

	const chain = "test-chain-bad-literal"

	create(t, chain)
	defer kill(t, chain)

	if err := testutil.FakeServiceDefinition("fake", `
chain = "blah-blah:blah"

[service]
name = "fake"
image = "`+path.Join(config.Global.DefaultRegistry, config.Global.ImageKeys)+`"
`); err != nil {
		t.Fatalf("can't create a fake service definition: %v", err)
	}

	if !util.Running(definitions.TypeChain, chain) {
		t.Fatalf("expecting test chain container")
	}

	do := definitions.NowDo()
	do.Operations.Args = []string{"fake"}
	do.ChainName = chain
	// [pv]: probably a bug. Bad literal chain link in a definition
	// file doesn't affect the service start. Links is not nil.
	if err := services.StartService(do); err != nil {
		t.Fatalf("expecting service to start, got %v", err)
	}

	links := testutil.Links("fake", definitions.TypeService)
	if len(links) != 1 || !strings.Contains(links[0], "/blah") {
		t.Fatalf("expected service be linked to a test chain, got %v", links)
	}
}

func TestServiceLinkChainedService(t *testing.T) {
	defer testutil.RemoveAllContainers()

	const chain = "test-chained-service"

	if err := testutil.FakeServiceDefinition("fake", `
chain = "$chain:fake"

[service]
name = "fake"
image = "`+path.Join(config.Global.DefaultRegistry, config.Global.ImageKeys)+`"

[dependencies]
services = [ "sham" ]
`); err != nil {
		t.Fatalf("can't create a fake service definition: %v", err)
	}

	if err := testutil.FakeServiceDefinition("sham", `
chain = "$chain:sham"

[service]
name = "sham"
image = "`+path.Join(config.Global.DefaultRegistry, config.Global.ImageKeys)+`"
data_container = true
`); err != nil {
		t.Fatalf("can't create a sham service definition: %v", err)
	}

	if util.Running(definitions.TypeChain, chain) {
		t.Fatalf("expecting test chain container doesn't run")
	}

	create(t, chain) // [zr] why was the NewChain here?
	defer kill(t, chain)

	if !util.Exists(definitions.TypeChain, chain) {
		t.Fatalf("expecting test chain container exists")
	}

	do := definitions.NowDo()
	do.Operations.Args = []string{"fake"}
	do.ChainName = chain
	if err := services.StartService(do); err != nil {
		t.Fatalf("expecting service to start, got %v", err)
	}

	if !util.Running(definitions.TypeService, "fake") {
		t.Fatalf("expecting fake service running")
	}
	if util.Exists(definitions.TypeData, "fake") {
		t.Fatalf("expecting fake data container doesn't exist")
	}
	if !util.Running(definitions.TypeService, "sham") {
		t.Fatalf("expecting sham service running")
	}
	if !util.Exists(definitions.TypeData, "sham") {
		t.Fatalf("expecting sham data container exist")
	}

	// [pv]: second service doesn't reference the chain.
	links := testutil.Links("fake", definitions.TypeService)

	if len(links) != 2 || !strings.Contains(strings.Join(links, " "), "/fake") || !strings.Contains(strings.Join(links, " "), "/sham") {
		t.Fatalf("expected service be linked to a test chain, got %v", links)
	}
}

func TestServiceLinkKeys(t *testing.T) {
	defer testutil.RemoveAllContainers()

	const chain = "chain-test-keys"
	create(t, chain)
	defer kill(t, chain)

	if !util.Exists(definitions.TypeChain, chain) {
		t.Fatalf("expecting test chain running")
	}

	do := definitions.NowDo()
	do.Operations.Args = []string{"keys"}
	do.ChainName = chain
	if err := services.StartService(do); err != nil {
		t.Fatalf("expecting service to start, got %v", err)
	}

	if !util.Running(definitions.TypeService, "keys") {
		t.Fatalf("expecting keys service running")
	}

	links := testutil.Links("keys", definitions.TypeService)
	if len(links) != 0 {
		t.Fatalf("expected service links be empty, got %v", links)
	}
}

func create(t *testing.T, chain string) {
	doMake := definitions.NowDo()
	doMake.Name = chain
	doMake.ChainType = "simplechain"
	if err := MakeChain(doMake); err != nil {
		t.Fatalf("expected a chain to be made, got %v", err)
	}

	do := definitions.NowDo()
	do.Name = chain
	do.Operations.PublishAllPorts = true
	do.Path = filepath.Join(common.ChainsPath, chain) // --init-dir
	if err := StartChain(do); err != nil {
		t.Fatalf("expected a new chain to be created, got %v", err)
	}
}

func start(t *testing.T, chain string) {
	do := definitions.NowDo()
	do.Name = chain
	do.Operations.PublishAllPorts = true
	if err := StartChain(do); err != nil {
		t.Fatalf("starting chain %v failed: %v", chain, err)
	}
}

func stop(t *testing.T, chain string) {
	do := definitions.NowDo()
	do.Name = chain
	do.Force = true
	if err := StopChain(do); err != nil {
		t.Fatalf("stopping chain %v failed: %v", chain, err)
	}
}

func kill(t *testing.T, chain string) {
	do := definitions.NowDo()
	do.Operations.Args, do.Rm, do.RmD = []string{"keys"}, true, true
	if err := services.KillService(do); err != nil {
		t.Fatalf("killing keys service failed: %v", err)
	}

	do = definitions.NowDo()
	do.Name, do.RmHF, do.RmD, do.Force = chain, true, true, true
	if err := RemoveChain(do); err != nil {
		t.Fatalf("killing chain failed: %v", err)
	}
}

func exec(t *testing.T, chain string, args []string) string {
	do := definitions.NowDo()
	do.Name = chain
	do.Operations.Args = args
	buf, err := ExecChain(do)
	if err != nil {
		log.Error(buf)
		t.Fatalf("expected chain to execute, got %v", err)
	}

	return buf.String()
}
