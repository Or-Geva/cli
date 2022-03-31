package main

import (
	clientTestUtils "github.com/jfrog/jfrog-client-go/utils/tests"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buger/jsonparser"
	"github.com/jfrog/jfrog-cli-core/v2/plugins"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	coreTests "github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/jfrog/jfrog-cli/plugins/commands/utils"
	"github.com/jfrog/jfrog-cli/utils/cliutils"
	"github.com/jfrog/jfrog-cli/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
)

const officialPluginForTest = "rt-fs"
const officialPluginVersion = "v1.0.0"
const customPluginName = "custom-plugin"

func TestPluginInstallUninstallOfficialRegistry(t *testing.T) {
	initPluginsTest(t)
	// Create temp jfrog home
	err, cleanUpJfrogHome := coreTests.SetJfrogHome()
	if err != nil {
		return
	}
	defer cleanUpJfrogHome()

	// Set empty plugins server to run against official registry.
	oldServer := os.Getenv(utils.PluginsServerEnv)
	defer func() {
		clientTestUtils.SetEnvAndAssert(t, utils.PluginsServerEnv, oldServer)
	}()
	clientTestUtils.SetEnvAndAssert(t, utils.PluginsServerEnv, "")
	oldRepo := os.Getenv(utils.PluginsRepoEnv)
	defer func() {
		clientTestUtils.SetEnvAndAssert(t, utils.PluginsRepoEnv, oldRepo)
	}()
	clientTestUtils.SetEnvAndAssert(t, utils.PluginsRepoEnv, "")
	jfrogCli := tests.NewJfrogCli(execMain, "jfrog", "")

	// Try installing a plugin with specific version .
	err = installAndAssertPlugin(t, jfrogCli, officialPluginForTest, officialPluginVersion)
	if err != nil {
		return
	}

	// Try installing the latest version of the plugin. Also verifies replacement was successful.
	err = installAndAssertPlugin(t, jfrogCli, officialPluginForTest, "")
	if err != nil {
		return
	}

	// Uninstall plugin from home dir.
	err = jfrogCli.Exec("plugin", "uninstall", officialPluginForTest)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	err = verifyPluginInPluginsDir(t, officialPluginForTest, false)
	if err != nil {
		return
	}
}

func installAndAssertPlugin(t *testing.T, jfrogCli *tests.JfrogCli, pluginName, pluginVersion string) error {
	// If version required, concat to plugin name
	identifier := pluginName
	if pluginVersion != "" {
		identifier += "@" + pluginVersion
	}

	// Install plugin from registry.
	err := jfrogCli.Exec("plugin", "install", identifier)
	if err != nil {
		assert.NoError(t, err)
		return err
	}
	err = verifyPluginInPluginsDir(t, pluginName, true)
	if err != nil {
		return err
	}

	err = verifyPluginSignature(t, jfrogCli)
	if err != nil {
		return err
	}

	return verifyPluginVersion(t, jfrogCli, pluginVersion)
}

func verifyPluginSignature(t *testing.T, jfrogCli *tests.JfrogCli) error {
	// Get signature from plugin.
	content, err := getCmdOutput(t, jfrogCli, officialPluginForTest, plugins.SignatureCommandName)
	if err != nil {
		return err
	}

	// Extract the name from the output.
	name, err := jsonparser.GetString(content, "name")
	if err != nil {
		assert.NoError(t, err)
		return err
	}
	assert.Equal(t, officialPluginForTest, name)

	// Extract the usage from the output.
	usage, err := jsonparser.GetString(content, "usage")
	if err != nil {
		assert.NoError(t, err)
		return err
	}
	assert.NotEmpty(t, usage)
	return nil
}

func verifyPluginVersion(t *testing.T, jfrogCli *tests.JfrogCli, expectedVersion string) error {
	// Run plugin's -v command.
	content, err := getCmdOutput(t, jfrogCli, officialPluginForTest, "-v")
	if err != nil {
		return err
	}
	if expectedVersion != "" {
		assert.NoError(t, utils.AssertPluginVersion(string(content), expectedVersion))
	}
	return err
}

func getCmdOutput(t *testing.T, jfrogCli *tests.JfrogCli, cmd ...string) ([]byte, error) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		assert.NoError(t, err)
		return nil, err
	}
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
		assert.NoError(t, r.Close())
	}()
	err = jfrogCli.Exec(cmd...)
	if err != nil {
		assert.NoError(t, err)
		assert.NoError(t, w.Close())
		return nil, err
	}
	err = w.Close()
	if err != nil {
		assert.NoError(t, err)
		return nil, err
	}
	content, err := ioutil.ReadAll(r)
	assert.NoError(t, err)
	return content, err
}

func verifyPluginInPluginsDir(t *testing.T, pluginName string, shouldExist bool) error {
	pluginsDir, err := coreutils.GetJfrogPluginsDir()
	if err != nil {
		assert.NoError(t, err)
		return err
	}

	actualExists, err := fileutils.IsFileExists(filepath.Join(pluginsDir, utils.GetLocalPluginExecutableName(pluginName)), false)
	if err != nil {
		assert.NoError(t, err)
		return err
	}
	if shouldExist {
		assert.True(t, actualExists, "expected plugin executable to be preset in plugins dir after installing")
	} else {
		assert.False(t, actualExists, "expected plugin executable not to be preset in plugins dir after uninstalling")
	}
	return nil
}

func initPluginsTest(t *testing.T) {
	if !*tests.TestPlugins {
		t.Skip("Skipping Plugins test. To run Plugins test add the '-test.plugins=true' option.")
	}
}

func TestPublishInstallCustomServer(t *testing.T) {
	initPluginsTest(t)
	// Create temp jfrog home
	err, cleanUpJfrogHome := coreTests.SetJfrogHome()
	if err != nil {
		return
	}
	defer cleanUpJfrogHome()

	jfrogCli := tests.NewJfrogCli(execMain, "jfrog", "")

	// Create server to use with the command.
	_, err = createServerConfigAndReturnPassphrase(t)
	defer deleteServerConfig(t)
	if err != nil {
		assert.NoError(t, err)
		return
	}

	// Set plugins server to run against the configured server.
	oldServer := os.Getenv(utils.PluginsServerEnv)
	defer func() {
		clientTestUtils.SetEnvAndAssert(t, utils.PluginsServerEnv, oldServer)
	}()
	clientTestUtils.SetEnvAndAssert(t, utils.PluginsServerEnv, tests.ServerId)
	oldRepo := os.Getenv(utils.PluginsRepoEnv)
	defer func() {
		clientTestUtils.SetEnvAndAssert(t, utils.PluginsRepoEnv, oldRepo)
	}()
	clientTestUtils.SetEnvAndAssert(t, utils.PluginsRepoEnv, tests.RtRepo1)

	err = setOnlyLocalArc(t)
	if err != nil {
		return
	}

	// Publish the CLI as a plugin to the registry.
	err = jfrogCli.Exec("plugin", "p", customPluginName, cliutils.GetVersion())
	if err != nil {
		assert.NoError(t, err)
		return
	}

	err = verifyPluginExistsInRegistry(t)
	if err != nil {
		return
	}

	// Install plugin from registry.
	err = jfrogCli.Exec("plugin", "install", customPluginName)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	err = verifyPluginInPluginsDir(t, customPluginName, true)
	if err != nil {
		return
	}
	pluginsDir, err := coreutils.GetJfrogPluginsDir()
	if err != nil {
		assert.NoError(t, err)
		return
	}
	clientTestUtils.RemoveAndAssert(t, filepath.Join(pluginsDir, utils.GetLocalPluginExecutableName(customPluginName)))
}

func verifyPluginExistsInRegistry(t *testing.T) error {
	searchFilePath, err := tests.CreateSpec(tests.SearchAllRepo1)
	if err != nil {
		assert.NoError(t, err)
		return err
	}
	localArc, err := utils.GetLocalArchitecture()
	if err != nil {
		assert.NoError(t, err)
		return err
	}
	expectedPath := utils.GetPluginPathInArtifactory(customPluginName, cliutils.GetVersion(), localArc)
	// Expected to find the plugin in the version and latest dir.
	expected := []string{
		expectedPath,
		strings.Replace(expectedPath, cliutils.GetVersion(), utils.LatestVersionName, 1),
	}
	verifyExistInArtifactory(expected, searchFilePath, t)
	return nil
}

// Set the local architecture to be the only one in map to avoid building for all architectures.
func setOnlyLocalArc(t *testing.T) error {
	localArcName, err := utils.GetLocalArchitecture()
	if err != nil {
		assert.NoError(t, err)
		return err
	}
	localArc := utils.ArchitecturesMap[localArcName]
	utils.ArchitecturesMap = map[string]utils.Architecture{
		localArcName: localArc,
	}
	return nil
}

func InitPluginsTests() {
	initArtifactoryCli()
	cleanUpOldRepositories()
	tests.AddTimestampToGlobalVars()
	createRequiredRepos()
}

func CleanPluginsTests() {
	deleteCreatedRepos()
}
