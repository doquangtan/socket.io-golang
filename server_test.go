package socketio

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandlerMessage(t *testing.T) {
	s := Io{
		namespaces: namespaces{
			list: map[string]*Namespace{
				"/": {Name: ""},
			},
		},
	}

	// NOTE: That one already passed before eef17c02d9676f5463453a5f167205cd4c4bbba1
	err := s.handlerMessage(&Socket{
		Id: "test", Nsp: "/",
	}, `42/ssh,["resize",{"cols":123,"rows":41}]`)

	t.Logf("%T %v", err, err)

	require.ErrorIs(t, err, ErrorInvalidConnection)
	// NOTE: That one would panic prior to eef17c02d9676f5463453a5f167205cd4c4bbba1
	require.NotPanics(t, func() {
		err := s.handlerMessage(&Socket{
			Id: "test", Nsp: "/",
		}, `42/logview,["resize",{"cols":124,"rows":34}]`)
		require.ErrorIs(t, err, ErrorInvalidConnection,
			"parsed content shouldn't cause an error",
		)
	}, "input content shouldn't cause a panics")
}
