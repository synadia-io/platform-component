package platform_component

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

type testData struct {
	Test string
	Ing  int
}

func startAPIServer(t testing.TB) (*server.Server, *httptest.Server) {
	t.Helper()

	natsServer, err := startNatsServer(t)
	if err != nil {
		t.Fatalf("failed to start nats server: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "only POST allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()

		req_body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}

		req_data := ConnectRequest{}
		err = json.Unmarshal(req_body, &req_data)
		if err != nil {
			http.Error(w, "failed to unmarshal body", http.StatusInternalServerError)
			return
		}

		connect := &testData{}

		err = json.Unmarshal(req_data.Data, connect)
		if err != nil {
			http.Error(w, "failed to unmarshal test config", http.StatusInternalServerError)
			return
		}

		if !nkeys.IsValidPublicUserKey(req_data.NKey) {
			http.Error(w, "failed to verify public user nkey", http.StatusInternalServerError)
			return
		}

		if connect.Test != "testing" {
			http.Error(w, "unexpected platform config", http.StatusInternalServerError)
			return
		}

		if connect.Ing != -1 {
			http.Error(w, "unexpected platform config", http.StatusInternalServerError)
			return
		}

		pc, err := json.Marshal(map[string]string{"Bucket": "bucket"})
		if err != nil {
			http.Error(w, "failed to marshall platform config", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		response := ConnectResponse{
			JWT:     "JWT123",
			Account: "CACCOUNT",
			Server:  natsServer.ClientURL(),
			Config:  pc,
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	return natsServer, httptest.NewServer(handler)
}

func startNatsServer(t testing.TB) (*server.Server, error) {
	t.Helper()
	require := require.New(t)

	s, err := server.NewServer(&server.Options{
		Port:      -1,
		JetStream: true,
		StoreDir:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("failed to create server: %v", err)
	}

	s.Start()
	time.Sleep(500 * time.Millisecond)

	nc, err := nats.Connect(s.ClientURL())
	require.NoError(err, "failed to connect to server")

	js, err := jetstream.New(nc)
	require.NoError(err, "failed to create jetstream context")

	_, err = js.CreateKeyValue(context.TODO(), jetstream.KeyValueConfig{
		Bucket: "bucket",
	})
	require.NoError(err, "failed to create kv")
	_ = nc.Drain()

	return s, nil
}

func TestRegister(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	natsserver, apiServer := startAPIServer(t)
	defer apiServer.Close()
	defer natsserver.Shutdown()

	c := Component("test", nil)
	err := c.Register("asdf", WithURL(apiServer.URL))
	assert.Contains(err.Error(), "failed to unmarshal test config")

	err = c.Register("asdf", WithURL(apiServer.URL), WithData(&testData{Test: "testing", Ing: -1}))
	require.Nil(err)
	require.NotEmpty(c.Config().Server)
	require.Equal("JWT123", c.Config().JWT)
	require.Equal("CACCOUNT", c.Config().Account)
	require.NotNil(c.Config().Config)
	require.NoError(c.Start(context.Background()))
	require.NoError(c.Stop())
}
