package proc

import (
	"fmt"
	"io"
	"net"

	"github.com/yezzey-gp/yproxy/pkg/storage"
	"github.com/yezzey-gp/yproxy/pkg/ylogger"
)

func ProcConn(s storage.StorageReader, c net.Conn) error {
	pr := NewProtoReader(c)
	tp, body, err := pr.ReadPacket()
	if err != nil {
		return err
	}

	ylogger.Zero.Debug().Str("msg-type", tp.String()).Msg("recieved client request")

	switch tp {
	case MessageTypeCat:
		name := GetCatName(body)
		ylogger.Zero.Debug().Str("object-path", name).Msg("cat object ")
		r, err := s.CatFileFromStorage(name)
		if err != nil {

			_, _ = c.Write([]byte(
				fmt.Sprintf("failed to compelete request: %v", err),
			))

			return err
		}
		io.Copy(c, r)

	default:
		_, err := c.Write([]byte(
			"wrong request type",
		))
		if err != nil {
			return err
		}

		return c.Close()
	}

	return nil
}
