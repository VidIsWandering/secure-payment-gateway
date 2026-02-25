package logger

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_StructuredJSON(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriter("info", &buf)

	log.Info().Str("key", "value").Msg("test message")

	var output map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &output)
	require.NoError(t, err, "logger output should be valid JSON")

	assert.Equal(t, "test message", output["message"])
	assert.Equal(t, "value", output["key"])
	assert.Equal(t, "info", output["level"])
	assert.Contains(t, output, "time", "should include timestamp")
}

func TestNew_DebugLevel(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriter("debug", &buf)

	log.Debug().Msg("debug msg")
	assert.NotEmpty(t, buf.String(), "debug messages should be logged at debug level")
}

func TestNew_InfoLevel_FiltersDebug(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriter("info", &buf)

	log.Debug().Msg("should not appear")
	assert.Empty(t, buf.String(), "debug messages should be filtered at info level")
}

func TestNew_ErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriter("error", &buf)

	log.Info().Msg("should not appear")
	assert.Empty(t, buf.String())

	log.Error().Msg("error msg")
	assert.NotEmpty(t, buf.String())
}

func TestNew_InvalidLevel_DefaultsToInfo(t *testing.T) {
	var buf bytes.Buffer
	log := NewWithWriter("invalid", &buf)

	log.Debug().Msg("should not appear")
	assert.Empty(t, buf.String(), "invalid level should default to info, filtering debug")

	log.Info().Msg("should appear")
	assert.NotEmpty(t, buf.String())
}

func TestNew_PrettyMode(t *testing.T) {
	// Just ensure it doesn't panic — pretty mode writes to stdout.
	log := New("info", true)
	log.Info().Msg("pretty mode test")
}
