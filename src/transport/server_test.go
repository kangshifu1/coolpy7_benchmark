package transport

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"packet"
)

func abstractServerTest(t *testing.T, protocol string) {
	server, err := testLauncher.Launch(protocol + "://localhost:0")
	require.NoError(t, err)

	go func() {
		conn1, err := server.Accept()
		require.NoError(t, err)

		pkt, err := conn1.Receive()
		assert.Equal(t, pkt.Type(), packet.CONNECT)
		assert.NoError(t, err)

		err = conn1.Send(packet.NewConnackPacket())
		assert.NoError(t, err)

		pkt, err = conn1.Receive()
		assert.Nil(t, pkt)
		assert.Equal(t, io.EOF, err)
	}()

	conn2, err := testDialer.Dial(getURL(server, protocol))
	require.NoError(t, err)

	err = conn2.Send(packet.NewConnectPacket())
	assert.NoError(t, err)

	pkt, err := conn2.Receive()
	assert.Equal(t, pkt.Type(), packet.CONNACK)
	assert.NoError(t, err)

	err = conn2.Close()
	assert.NoError(t, err)

	err = server.Close()
	assert.NoError(t, err)
}

func abstractServerLaunchErrorTest(t *testing.T, protocol string) {
	server, err := testLauncher.Launch(protocol + "://localhost:1")
	assert.Error(t, err)
	assert.Nil(t, server)
}

func abstractServerAcceptAfterCloseTest(t *testing.T, protocol string) {
	server, err := testLauncher.Launch(protocol + "://localhost:0")
	require.NoError(t, err)

	err = server.Close()
	assert.NoError(t, err)

	conn, err := server.Accept()
	assert.Nil(t, conn)
	assert.Error(t, err)
}

func abstractServerCloseAfterCloseTest(t *testing.T, protocol string) {
	server, err := testLauncher.Launch(protocol + "://localhost:0")
	require.NoError(t, err)

	err = server.Close()
	assert.NoError(t, err)

	err = server.Close()
	assert.Error(t, err)
}

func abstractServerAddrTest(t *testing.T, protocol string) {
	server, err := testLauncher.Launch(protocol + "://localhost:0")
	require.NoError(t, err)

	assert.Equal(t, fmt.Sprintf("127.0.0.1:%s", getPort(server)), server.Addr().String())

	err = server.Close()
	assert.NoError(t, err)
}
