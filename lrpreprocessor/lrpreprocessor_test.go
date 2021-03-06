package lrpreprocessor_test

import (
	"errors"

	. "github.com/cloudfoundry-incubator/converger/lrpreprocessor"
	"github.com/cloudfoundry-incubator/runtime-schema/bbs/fake_bbs"
	"github.com/cloudfoundry-incubator/runtime-schema/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("LRPreProcessor", func() {
	var (
		bbs                 *fake_bbs.FakeConvergerBBS
		lrpp                *LRPreProcessor
		lrpWithPlaceholders models.DesiredLRP
		expectedLRP         models.DesiredLRP
		preProcessedLRP     models.DesiredLRP
		preProcessErr       error
	)

	BeforeEach(func() {
		bbs = fake_bbs.NewFakeConvergerBBS()
		lrpp = New(bbs)

		lrpWithPlaceholders = models.DesiredLRP{
			Domain:      "some-domain",
			ProcessGuid: "some-process-guid",

			Stack: "some-stack",

			Log: models.LogConfig{
				Guid:       "some-log-guid",
				SourceName: "App",
			},

			Actions: []models.ExecutorAction{
				{
					Action: models.DownloadAction{
						From:    "PLACEHOLDER_FILESERVER_URL/some-download/path",
						To:      "/tmp/some-download",
						Extract: true,
					},
				},
				models.Parallel(
					models.ExecutorAction{
						models.RunAction{
							Path: "some-path-to-run",
						},
					},
					models.ExecutorAction{
						models.MonitorAction{
							Action: models.ExecutorAction{
								models.RunAction{
									Path: "ls",
									Args: []string{"-al"},
								},
							},
							HealthyThreshold:   1,
							UnhealthyThreshold: 1,
							HealthyHook: models.HealthRequest{
								Method: "PUT",
								URL:    "http://example.com/oh-yes/PLACEHOLDER_INSTANCE_INDEX/foo/PLACEHOLDER_INSTANCE_GUID",
							},
							UnhealthyHook: models.HealthRequest{
								Method: "PUT",
								URL:    "http://example.com/oh-no/PLACEHOLDER_INSTANCE_INDEX/foo/PLACEHOLDER_INSTANCE_GUID",
							},
						},
					},
				),
			},
		}

		expectedLRP = models.DesiredLRP{
			Domain:      "some-domain",
			ProcessGuid: "some-process-guid",

			Stack: "some-stack",

			Log: models.LogConfig{
				Guid:       "some-log-guid",
				SourceName: "App",
			},

			Actions: []models.ExecutorAction{
				{
					Action: models.DownloadAction{
						From:    "http://some-fake-file-server/some-download/path",
						To:      "/tmp/some-download",
						Extract: true,
					},
				},
				models.Parallel(
					models.ExecutorAction{
						models.RunAction{
							Path: "some-path-to-run",
						},
					},
					models.ExecutorAction{
						models.MonitorAction{
							Action: models.ExecutorAction{
								models.RunAction{
									Path: "ls",
									Args: []string{"-al"},
								},
							},
							HealthyThreshold:   1,
							UnhealthyThreshold: 1,
							HealthyHook: models.HealthRequest{
								Method: "PUT",
								URL:    "http://example.com/oh-yes/2/foo/some-instance-guid",
							},
							UnhealthyHook: models.HealthRequest{
								Method: "PUT",
								URL:    "http://example.com/oh-no/2/foo/some-instance-guid",
							},
						},
					},
				),
			},
		}
	})

	JustBeforeEach(func() {
		preProcessedLRP, preProcessErr = lrpp.PreProcess(lrpWithPlaceholders, 2, "some-instance-guid")
	})

	Context("when a file server is available", func() {
		It("replaces all placeholders with their actual values", func() {
			Ω(preProcessedLRP.Actions[1]).Should(Equal(expectedLRP.Actions[1]))
		})

		It("does not return an error", func() {
			Ω(preProcessErr).ShouldNot(HaveOccurred())
		})
	})

	Context("when no file servers are available", func() {
		var expectedErr = errors.New("ahhhh!")

		BeforeEach(func() {
			bbs.FileServerGetter.WhenGettingAvailableFileServer = func() (string, error) {
				return "", expectedErr
			}
		})

		It("returns an error", func() {
			Ω(preProcessErr).Should(Equal(expectedErr))
		})
	})
})
