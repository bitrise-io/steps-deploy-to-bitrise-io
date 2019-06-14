package xcresult3

import (
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/xcode-project/serialized"
	"github.com/bitrise-steplib/steps-deploy-to-bitrise-io/test/junit"
)

// Converter ...
type Converter struct {
	xcresultPth string
}

func majorVersion(document serialized.Object) (int, error) {
	version, err := document.Object("version")
	if err != nil {
		return -1, err
	}

	major, err := version.Value("major")
	if err != nil {
		return -1, err
	}
	return major.(int), nil
}

func documentMajorVersion(pth string) (int, error) {
	content, err := fileutil.ReadBytesFromFile(pth)
	if err != nil {
		return -1, err
	}

	var info serialized.Object
	if err := json.Unmarshal(content, &info); err != nil {
		return -1, err
	}

	return majorVersion(info)
}

// Detect ...
func (c *Converter) Detect(files []string) bool {
	for _, file := range files {
		if filepath.Ext(file) != ".xcresult" {
			continue
		}

		infoPth := filepath.Join(file, "Info.plist")
		if exist, err := pathutil.IsPathExists(infoPth); err != nil || exist == false {
			continue
		}

		version, err := documentMajorVersion(infoPth)
		if err != nil {
			continue
		}

		if version < 3 {
			continue
		}

		c.xcresultPth = file
		return true
	}
	return false
}

// XML ...
func (c *Converter) XML() (junit.XML, error) {
	testResultDir := filepath.Dir(c.xcresultPth)

	record, summaries, err := Parse(c.xcresultPth)
	if err != nil {
		return junit.XML{}, err
	}

	var xmlData junit.XML
	for _, summary := range summaries {
		testsByName := summary.tests()

		for name, tests := range testsByName {
			testSuit := junit.TestSuite{
				Name:     name,
				Tests:    len(tests),
				Failures: summary.failuresCount(name),
				Time:     summary.totalTime(name),
			}

			for _, test := range tests {
				var duartion float64
				if test.Duration.Value != "" {
					duartion, _ = strconv.ParseFloat(test.Duration.Value, 64)
				}

				testSuit.TestCases = append(testSuit.TestCases, junit.TestCase{
					Name:      test.Name.Value,
					ClassName: strings.Split(test.Identifier.Value, "/")[0],
					Failure:   record.failure(test),
					Time:      duartion,
				})

				if err := test.exportScreenshots(c.xcresultPth, testResultDir); err != nil {
					return junit.XML{}, err
				}
			}

			xmlData.TestSuites = append(xmlData.TestSuites, testSuit)
		}
	}

	return xmlData, nil
}