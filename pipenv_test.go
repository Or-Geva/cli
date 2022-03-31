package main

import (
	clientTestUtils "github.com/jfrog/jfrog-client-go/utils/tests"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli/inttestutils"
	"github.com/jfrog/jfrog-cli/utils/tests"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPipenvInstall(t *testing.T) {
	// Init pipenv test.
	initPipenvTest(t)

	// Populate cli config with 'default' servvver.
	oldHomeDir, newHomeDir := prepareHomeDir(t)
	defer func() {
		assert.NoError(t, os.Setenv(coreutils.HomeDir, oldHomeDir))
		assert.NoError(t, os.RemoveAll(newHomeDir))
	}()

	// Create test cases.
	allTests := []struct {
		name                 string
		project              string
		outputFolder         string
		moduleId             string
		args                 []string
		expectedDependencies int
		cleanAfterExecution  bool
	}{
		{"pipenv-with-module", "pipenvproject", "pipenv-with-module", "pipenv-with-module", []string{"pipenv", "install", "--build-name=" + tests.PipenvBuildName, "--module=pipenv-with-module"}, 3, true},
	}

	// Run test cases.
	for buildNumber, test := range allTests {
		t.Run(test.name, func(t *testing.T) {
			testPipenvCmd(t, createPipenvProject(t, test.outputFolder, test.project), strconv.Itoa(buildNumber), test.moduleId, test.expectedDependencies, test.args)
			if test.cleanAfterExecution {
				// cleanup
				inttestutils.DeleteBuild(serverDetails.ArtifactoryUrl, tests.PipenvBuildName, artHttpDetails)
			}
		})
	}
	tests.CleanFileSystem()
}

func testPipenvCmd(t *testing.T, projectPath, buildNumber, module string, expectedDependencies int, args []string) {
	wd, err := os.Getwd()
	assert.NoError(t, err, "Failed to get current dir")
	chdirCallback := clientTestUtils.ChangeDirWithCallback(t, wd, projectPath)
	defer chdirCallback()

	args = append(args, "--build-number="+buildNumber)

	jfrogCli := tests.NewJfrogCli(execMain, "jfrog", "")
	err = jfrogCli.WithoutCredentials().Exec(args...)
	if err != nil {
		assert.Fail(t, "Failed executing pipenv-install command", err.Error())
		return
	}

	inttestutils.ValidateGeneratedBuildInfoModule(t, tests.PipenvBuildName, buildNumber, "", []string{module}, buildinfo.Python)
	assert.NoError(t, artifactoryCli.Exec("bp", tests.PipenvBuildName, buildNumber))

	publishedBuildInfo, found, err := tests.GetBuildInfo(serverDetails, tests.PipenvBuildName, buildNumber)
	if err != nil {
		assert.NoError(t, err)
		return
	}
	if !found {
		assert.True(t, found, "build info was expected to be found")
		return
	}

	buildInfo := publishedBuildInfo.BuildInfo
	require.NotEmpty(t, buildInfo.Modules, "Pipenv build info was not generated correctly, no modules were created.")
	assert.Len(t, buildInfo.Modules[0].Dependencies, expectedDependencies, "Incorrect number of artifacts found in the build-info")
	assert.Equal(t, module, buildInfo.Modules[0].Id, "Unexpected module name")
}

func createPipenvProject(t *testing.T, outFolder, projectName string) string {
	projectSrc := filepath.Join(filepath.FromSlash(tests.GetTestResourcesPath()), "pipenv", projectName)
	projectTarget := filepath.Join(tests.Out, outFolder+"-"+projectName)
	err := fileutils.CreateDirIfNotExist(projectTarget)
	assert.NoError(t, err)

	// Copy pipenv-installation file.
	err = fileutils.CopyDir(projectSrc, projectTarget, true, nil)
	assert.NoError(t, err)

	// Copy pipenv-config file.
	configSrc := filepath.Join(filepath.FromSlash(tests.GetTestResourcesPath()), "pipenv", "pipenv.yaml")
	configTarget := filepath.Join(projectTarget, ".jfrog", "projects")
	_, err = tests.ReplaceTemplateVariables(configSrc, configTarget)
	assert.NoError(t, err)

	return projectTarget
}

func initPipenvTest(t *testing.T) {
	if !*tests.TestPipenv {
		t.Skip("Skipping Pipenv test. To run Pipenv test add the '-test.pipenv=true' option.")
	}
	require.True(t, isRepoExist(tests.PipenvRemoteRepo), "Pypi test remote repository doesn't exist.")
	require.True(t, isRepoExist(tests.PipenvVirtualRepo), "Pypi test virtual repository doesn't exist.")
}
