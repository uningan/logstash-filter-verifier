package main_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matryer/is"
	"github.com/spf13/viper"

	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/app/daemon"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/app/daemon/run"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/api/grpc"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/daemon/file"
	"github.com/magnusbaeck/logstash-filter-verifier/v2/internal/logging"
)

func TestIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") != "1" {
		t.Skip("integration test skipped, enable with env var `INTEGRATION_TEST=1`")
	}

	is := is.New(t)

	viper.SetConfigName("logstash-filter-verifier")
	viper.AddConfigPath(".")

	viper.SetDefault("logstash.path", "/usr/share/logstash/bin/logstash")

	// Read config
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			t.Fatalf("Error processing config file: %v", err)
		}
	}

	testLogger := &logging.LoggerMock{
		DebugFunc:    func(args ...interface{}) {},
		DebugfFunc:   func(format string, args ...interface{}) {},
		ErrorFunc:    func(args ...interface{}) { t.Log(args...) },
		ErrorfFunc:   func(format string, args ...interface{}) { t.Logf(format, args...) },
		FatalFunc:    func(args ...interface{}) { t.Log(args...) },
		FatalfFunc:   func(format string, args ...interface{}) { t.Logf(format, args...) },
		InfoFunc:     func(args ...interface{}) { t.Log(args...) },
		InfofFunc:    func(format string, args ...interface{}) { t.Logf(format, args...) },
		WarningFunc:  func(args ...interface{}) { t.Log(args...) },
		WarningfFunc: func(format string, args ...interface{}) { t.Logf(format, args...) },
	}
	logging.SetLevel("INFO")

	if os.Getenv("INTEGRATION_TEST_DEBUG") == "1" {
		testLogger.DebugFunc = func(args ...interface{}) { t.Log(args...) }
		testLogger.DebugfFunc = func(format string, args ...interface{}) { t.Logf(format, args...) }
		logging.SetLevel("DEBUG")
	}

	tempdir := t.TempDir()
	// Start Daemon
	socket := filepath.Join(tempdir, "integration_test.socket")
	logstashPath := viper.GetString("logstash.path")
	if !file.Exists(logstashPath) {
		t.Fatalf("Logstash needs to be present in %q for the integration tests to work", logstashPath)
	}

	log := testLogger
	server := daemon.New(socket, logstashPath, nil, log, 10*time.Second, 3*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	go func() {
		defer cancel()

		is := is.New(t)

		defer server.Cleanup()

		err := server.Run(ctx)
		is.NoErr(err)
	}()

	i := 0
	for {
		if file.Exists(socket) {
			break
		}
		time.Sleep(100 * time.Millisecond)
		i++
		if i >= 20 {
			t.Fatalf("wait for socket file failed")
		}
	}

	// Run tests
	cases := []struct {
		name  string
		debug bool

		// optional integration tests require additional logstash plugins,
		// which are not provided by a default installation.
		optional bool
	}{
		{
			name: "basic_pipeline",
		},
		{
			name: "conditional_output",
		},
		{
			name: "pipeline_to_pipeline",
		},
		{
			name:  "basic_pipeline_debug",
			debug: true,
		},
		{
			name: "codec_test",
		},
		{
			name:     "codec_optional_test",
			optional: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)

			if tc.optional && os.Getenv("INTEGRATION_TEST_OPTIONAL") != "1" {
				t.Skipf("optional integration test %q skipped, enable with env var `INTEGRATION_TEST_OPTIONAL=1`", tc.name)
			}
			client, err := run.New(
				filepath.Join(tempdir, "integration_test.socket"),
				log,
				"testdata/"+tc.name+".yml",
				"testdata/"+tc.name,
				"testdata/testcases/"+tc.name,
				"@metadata",
				tc.debug,
			)
			is.NoErr(err)

			err = client.Run()
			is.NoErr(err)
		})
	}

	_, err := server.Shutdown(context.Background(), &grpc.ShutdownRequest{})
	is.NoErr(err)

	<-ctx.Done()
}