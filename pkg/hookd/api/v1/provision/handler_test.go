package api_v1_provision_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/navikt/deployment/pkg/azure/graphapi"
	"github.com/navikt/deployment/pkg/hookd/api"
	"github.com/navikt/deployment/pkg/hookd/api/v1"
	"github.com/navikt/deployment/pkg/hookd/api/v1/provision"
	"github.com/navikt/deployment/pkg/hookd/database"
	"github.com/stretchr/testify/assert"
)

var (
	secretKey    = api_v1.Key{0xab, 0xcd, 0xef} // abcdef
	provisionKey = []byte("cryptographically secure")
)

type request struct {
	Headers map[string]string
	Body    json.RawMessage
}

type response struct {
	StatusCode int                       `json:"statusCode"`
	Body       api_v1_provision.Response `json:"body"`
}

type testCase struct {
	Request  request  `json:"request"`
	Response response `json:"response"`
}

type apiKeyStorage struct {
}

type teamClient struct {
}

func (a *apiKeyStorage) ApiKeys(ctx context.Context, team string) (database.ApiKeys, error) {
	switch team {
	case "new", "unwritable":
		return nil, database.ErrNotFound
	case "unavailable":
		return nil, fmt.Errorf("service unavailable")
	default:
		return []database.ApiKey{{
			Key:     secretKey,
			Expires: time.Now().Add(1 * time.Hour),
		}}, nil
	}
}

func (a *apiKeyStorage) RotateApiKey(ctx context.Context, team, groupId string, key []byte) error {
	switch team {
	case "unwritable", "unwritable_with_rotate":
		return fmt.Errorf("service unavailable")
	default:
		return nil
	}
}

func (t *teamClient) Team(ctx context.Context, name string) (*graphapi.Team, error) {
	switch name {
	default:
		return &graphapi.Team{}, nil
	}
}

func (t *teamClient) IsErrNotFound(err error) bool {
	return err == graphapi.ErrNotFound
}

func testStatusResponse(t *testing.T, recorder *httptest.ResponseRecorder, response response) {
	assert.Equal(t, response.StatusCode, recorder.Code)
	if response.StatusCode == http.StatusNoContent {
		return
	}

	decodedBody := api_v1_provision.Response{}
	err := json.Unmarshal(recorder.Body.Bytes(), &decodedBody)
	assert.NoError(t, err)
	assert.Equal(t, response.Body.Message, decodedBody.Message)
}

// Inject timestamp in request payload
func addTimestampToBody(in []byte, timeshift int64) []byte {
	tmp := make(map[string]interface{})
	err := json.Unmarshal(in, &tmp)
	if err != nil {
		return in
	}
	if _, ok := tmp["timestamp"]; ok {
		// timestamp already provided in test fixture
		return in
	}
	tmp["timestamp"] = time.Now().Unix() + timeshift
	out, err := json.Marshal(tmp)
	if err != nil {
		return in
	}
	return out
}

func fileReader(file string) io.Reader {
	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	return f
}

func statusSubTest(t *testing.T, name string) {
	inFile := fmt.Sprintf("testdata/%s", name)

	fixture := fileReader(inFile)
	data, err := ioutil.ReadAll(fixture)
	if err != nil {
		t.Error(data)
		t.Fail()
	}

	test := testCase{}
	err = json.Unmarshal(data, &test)
	if err != nil {
		t.Error(string(data))
		t.Fail()
	}

	body := addTimestampToBody(test.Request.Body, 0)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/v1/provision", bytes.NewReader(body))
	request.Header.Set("content-type", "application/json")

	for key, val := range test.Request.Headers {
		request.Header.Set(key, val)
	}

	// Generate HMAC header for cases where the header should be valid
	if len(request.Header.Get(api_v1.SignatureHeader)) == 0 {
		mac := api_v1.GenMAC(body, provisionKey)
		request.Header.Set(api_v1.SignatureHeader, hex.EncodeToString(mac))
	}

	apiKeyStore := &apiKeyStorage{}
	teamClient := &teamClient{}

	handler := api.New(api.Config{
		ApiKeyStore:  apiKeyStore,
		MetricsPath:  "/metrics",
		ProvisionKey: provisionKey,
		TeamClient:   teamClient,
	})

	handler.ServeHTTP(recorder, request)

	testStatusResponse(t, recorder, test.Response)
}

func TestHandler(t *testing.T) {
	files, err := ioutil.ReadDir("testdata")
	if err != nil {
		t.Error(err)
		t.Fail()
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		t.Run(name, func(t *testing.T) {
			statusSubTest(t, name)
		})
	}
}
