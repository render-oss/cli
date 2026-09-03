package logs

import (
	"errors"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestTransientStreamFailureClassification(t *testing.T) {
	assert.True(t, isTransientStreamFailure(&websocket.CloseError{Code: websocket.CloseTryAgainLater}))
	assert.False(t, isTransientStreamFailure(&websocket.CloseError{Code: websocket.CloseNormalClosure}))
	assert.False(t, isTransientStreamFailure(errors.New("invalid log payload")))
}
