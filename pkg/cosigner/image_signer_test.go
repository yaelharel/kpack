package cosigner

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/buildpacks/lifecycle"
	"github.com/sclevine/spec"
	"github.com/sigstore/cosign/cmd/cosign/cli"
	"github.com/stretchr/testify/assert"
)

func TestImageSigner(t *testing.T) {
	spec.Run(t, "Test Cosign Image Signer Main", testImageSigner)
}

func testImageSigner(t *testing.T, when spec.G, it spec.S) {
	var (
		logger = log.New(ioutil.Discard, "", 0)

		signer = NewImageSigner(logger)
		report lifecycle.ExportReport
	)

	when("#Sign", func() {
		it.Before(func() {
			// Override secretLocation for test
			secretLocation = createCosignKeyFiles(t)

			report = createReportToml(t, secretLocation)

			os.Unsetenv(cosignRepositoryEnv)
			os.Unsetenv(cosignDockerMediaTypesEnv)
		})

		it("signs images", func() {
			cliSignCmdCallCount := 0
			cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
				t.Helper()
				assert.Equal(t, "example-registry.io/test:latest", imageRef)
				assert.Contains(t, ko.KeyRef, "cosign.key")
				assert.Contains(t, ko.KeyRef, secretLocation)

				password, err := ko.PassFunc(true)
				assert.Nil(t, err)

				assert.Equal(t, []byte(""), password)
				assert.Nil(t, annotations)
				cliSignCmdCallCount++
				return nil
			}

			err := signer.Sign(report, nil, nil, nil)
			assert.Nil(t, err)

			assert.Equal(t, 2, cliSignCmdCallCount)
		})

		it("signs images with key password", func() {
			firstPassword := []byte("secretPass1")
			secondPassword := []byte("secretPass2")
			ioutil.WriteFile(fmt.Sprintf("%s/secret-name-1/cosign.password", secretLocation), firstPassword, 0644)
			ioutil.WriteFile(fmt.Sprintf("%s/secret-name-2/cosign.password", secretLocation), secondPassword, 0644)

			cliSignCmdCallCount := 0
			firstPasswordSeenCount := 0
			secondPasswordSeenCount := 0

			cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
				t.Helper()
				assert.Equal(t, "example-registry.io/test:latest", imageRef)
				assert.Contains(t, ko.KeyRef, "cosign.key")
				assert.Contains(t, ko.KeyRef, secretLocation)

				password, err := ko.PassFunc(true)
				assert.Nil(t, err)

				assert.Contains(t, [][]byte{firstPassword, secondPassword}, password)

				if string(firstPassword) == string(password) {
					firstPasswordSeenCount++
				}

				if string(secondPassword) == string(password) {
					secondPasswordSeenCount++
				}

				assert.Nil(t, annotations)
				cliSignCmdCallCount++
				return nil
			}

			err := signer.Sign(report, nil, nil, nil)
			assert.Nil(t, err)

			assert.Equal(t, 2, cliSignCmdCallCount)
			assert.Equal(t, 1, firstPasswordSeenCount)
			assert.Equal(t, 1, secondPasswordSeenCount)
		})

		it("signs with annotations", func() {
			expectedAnnotation := map[string]interface{}{
				"annotationKey1": "value1",
			}

			cliSignCmdCallCount := 0
			cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
				t.Helper()
				assert.Equal(t, "example-registry.io/test:latest", imageRef)
				assert.Contains(t, ko.KeyRef, "cosign.key")
				assert.Contains(t, ko.KeyRef, secretLocation)
				assert.Equal(t, expectedAnnotation, annotations)
				cliSignCmdCallCount++
				return nil
			}

			err := signer.Sign(report, expectedAnnotation, nil, nil)
			assert.Nil(t, err)

			assert.Equal(t, 2, cliSignCmdCallCount)
		})

		it("errors when signing fails", func() {
			cliSignCmdCallCount := 0

			cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
				cliSignCmdCallCount++
				return fmt.Errorf("fake error")
			}

			expectedErrorMessage := fmt.Sprintf("unable to sign image with %s/secret-name-1/cosign.key: fake error", secretLocation)
			err := signer.Sign(report, nil, nil, nil)
			assert.Error(t, err)
			assert.Equal(t, expectedErrorMessage, err.Error())
			assert.Equal(t, 1, cliSignCmdCallCount)
		})

		it("sets COSIGN_REPOSITORY environment variable", func() {
			cliSignCmdCallCount := 0

			assert.Empty(t, len(os.Getenv(cosignRepositoryEnv)))
			cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
				t.Helper()
				if cliSignCmdCallCount == 0 {
					assert.Equal(t, "registry.example.com/fakeproject", os.Getenv(cosignRepositoryEnv))
				} else {
					assertUnset(t, cosignRepositoryEnv)
				}
				cliSignCmdCallCount++
				return nil
			}

			cosignRepositories := map[string]interface{}{
				"secret-name-1": "registry.example.com/fakeproject",
			}

			err := signer.Sign(report, nil, cosignRepositories, nil)
			assert.Nil(t, err)
			assert.Equal(t, 2, cliSignCmdCallCount)

			assertUnset(t, cosignRepositoryEnv)
		})

		it("sets COSIGN_DOCKER_MEDIA_TYPES environment variable", func() {
			cliSignCmdCallCount := 0

			assertUnset(t, cosignDockerMediaTypesEnv)
			cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
				t.Helper()
				if cliSignCmdCallCount == 0 {
					assert.Equal(t, "1", os.Getenv(cosignDockerMediaTypesEnv))
				} else {
					assertUnset(t, cosignDockerMediaTypesEnv)
				}

				cliSignCmdCallCount++
				return nil
			}

			cosignDockerMediaTypes := map[string]interface{}{
				"secret-name-1": "1",
			}

			err := signer.Sign(report, nil, nil, cosignDockerMediaTypes)
			assert.Nil(t, err)
			assert.Equal(t, 2, cliSignCmdCallCount)

			assertUnset(t, cosignDockerMediaTypesEnv)
		})

		it("sets both COSIGN_REPOSITORY and COSIGN_DOCKER_MEDIA_TYPES environment variable", func() {
			cliSignCmdCallCount := 0

			assertUnset(t, cosignDockerMediaTypesEnv)
			assertUnset(t, cosignRepositoryEnv)
			cliSignCmd = func(ctx context.Context, ko cli.KeyOpts, annotations map[string]interface{}, imageRef, certPath string, upload bool, payloadPath string, force, recursive bool) error {
				t.Helper()
				assert.Equal(t, "1", os.Getenv(cosignDockerMediaTypesEnv))
				assert.Equal(t, "registry.example.com/fakeproject", os.Getenv(cosignRepositoryEnv))
				cliSignCmdCallCount++
				return nil
			}

			cosignRepositories := map[string]interface{}{
				"secret-name-1": "registry.example.com/fakeproject",
				"secret-name-2": "registry.example.com/fakeproject",
			}

			cosignDockerMediaTypes := map[string]interface{}{
				"secret-name-1": "1",
				"secret-name-2": "1",
			}

			err := signer.Sign(report, nil, cosignRepositories, cosignDockerMediaTypes)
			assert.Nil(t, err)
			assert.Equal(t, 2, cliSignCmdCallCount)

			assertUnset(t, cosignDockerMediaTypesEnv)
			assertUnset(t, cosignRepositoryEnv)
		})
	})
}

func createCosignKeyFiles(t *testing.T) string {
	secretLocation = t.TempDir()

	os.Mkdir(fmt.Sprintf("%s/secret-name-1", secretLocation), 0700)
	os.Mkdir(fmt.Sprintf("%s/secret-name-2", secretLocation), 0700)

	os.Create(fmt.Sprintf("%s/secret-name-1/cosign.key", secretLocation))
	os.Create(fmt.Sprintf("%s/secret-name-2/cosign.key", secretLocation))

	return secretLocation
}

func createReportToml(t *testing.T, secretLocation string) lifecycle.ExportReport {
	var report lifecycle.ExportReport
	_, err := toml.Decode(`[image]
	tags = ["example-registry.io/test:latest", "example-registry.io/test:other-tag"]`, &report)
	assert.Nil(t, err)
	return report
}

func assertUnset(t *testing.T, envName string, msg ...string) {
	value, isSet := os.LookupEnv(envName)
	assert.False(t, isSet)
	assert.Equal(t, "", value)
}
