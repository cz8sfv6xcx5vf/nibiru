package bindings_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/NibiruChain/nibiru/app"
	"github.com/NibiruChain/nibiru/wasmbinding/bindings"
)

type TestSuiteBindingJsonTypes struct {
	suite.Suite
	fileJson map[string]json.RawMessage
}

func TestSuiteBindingJsonTypes_RunAll(t *testing.T) {
	suite.Run(t, new(TestSuiteBindingJsonTypes))
}

func (s *TestSuiteBindingJsonTypes) SetupSuite() {
	app.SetPrefixes(app.AccountAddressPrefix)
	file, err := os.Open("query_resp.json")
	s.NoError(err)
	defer file.Close()

	var fileJson map[string]json.RawMessage
	err = json.NewDecoder(file).Decode(&fileJson)
	s.NoError(err)
	s.fileJson = fileJson
}

func getFileJson(t *testing.T) (fileJson map[string]json.RawMessage) {
	file, err := os.Open("execute_msg.json")
	require.NoError(t, err)
	defer file.Close()

	err = json.NewDecoder(file).Decode(&fileJson)
	require.NoError(t, err)
	return fileJson
}

func (s *TestSuiteBindingJsonTypes) TestExecuteMsgs() {
	t := s.T()
	fileJson := getFileJson(t)

	testCaseMap := []string{
		"donate_to_insurance_fund",
		"edit_oracle_params",
		"set_market_enabled",
		"insurance_fund_withdraw",
		"create_market",
		"no_op",
	}

	for _, name := range testCaseMap {
		t.Run(name, func(t *testing.T) {
			var bindingMsg bindings.NibiruMsg
			err := json.Unmarshal(fileJson[name], &bindingMsg)
			assert.NoErrorf(t, err, "name: %v", name)

			jsonBz, err := json.Marshal(bindingMsg)
			assert.NoErrorf(t, err, "jsonBz: %s", jsonBz)

			// Json files are not compacted, so we need to compact them before comparing
			compactJsonBz, err := compactJsonData(jsonBz)
			require.NoError(t, err)

			fileBytes, err := fileJson[name].MarshalJSON()
			require.NoError(t, err)
			compactFileBytes, err := compactJsonData(fileBytes)
			require.NoError(t, err)

			var reconsitutedBindingMsg bindings.NibiruMsg
			err = json.Unmarshal(compactFileBytes.Bytes(), &reconsitutedBindingMsg)
			require.NoError(t, err)

			compactFileStr := compactFileBytes.String()
			compactJsonStr := compactJsonBz.String()
			require.EqualValuesf(
				t, bindingMsg, reconsitutedBindingMsg,
				"compactFileStr: %s\ncompactJsonStr: ", compactFileStr, compactJsonStr,
			)
		})
	}
}

func compactJsonData(data []byte) (*bytes.Buffer, error) {
	compactData := new(bytes.Buffer)
	err := json.Compact(compactData, data)
	return compactData, err
}
