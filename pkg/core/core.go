package core

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yezzey-gp/yproxy/config"
	"github.com/yezzey-gp/yproxy/pkg/client"
	"github.com/yezzey-gp/yproxy/pkg/clientpool"
	"github.com/yezzey-gp/yproxy/pkg/crypt"
	"github.com/yezzey-gp/yproxy/pkg/proc"
	"github.com/yezzey-gp/yproxy/pkg/sdnotifier"
	"github.com/yezzey-gp/yproxy/pkg/storage"
	"github.com/yezzey-gp/yproxy/pkg/ylogger"
)

type Instance struct {
	pool clientpool.Pool
}

func NewInstance() *Instance {
	return &Instance{
		pool: clientpool.NewClientPool(),
	}
}

func (i *Instance) Run(instanceCnf *config.Instance) error {

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	go func() {
		defer os.Remove(instanceCnf.SocketPath)
		defer cancelCtx()

		for {
			s := <-sigs
			ylogger.Zero.Info().Str("signal", s.String()).Msg("received signal")

			switch s {
			case syscall.SIGUSR1:
				ylogger.ReloadLogger(instanceCnf.LogPath)
			case syscall.SIGUSR2:
				return
			case syscall.SIGHUP:
				// reread config file

			case syscall.SIGINT, syscall.SIGTERM:

				// make better
				return
			default:
				return
			}
		}
	}()

	/* dispatch statistic server */
	go func() {

		listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%v", instanceCnf.StatPort))
		if err != nil {
			ylogger.Zero.Error().Err(err).Msg("failed to start socket listener")
			return
		}
		defer listener.Close()

		for {
			clConn, err := listener.Accept()
			if err != nil {
				ylogger.Zero.Error().Err(err).Msg("failed to accept connection")
			}
			ylogger.Zero.Debug().Str("addr", clConn.LocalAddr().String()).Msg("accepted client connection")

			clConn.Write([]byte("Hello from stats server!!\n"))
			clConn.Write([]byte("Client id | Optype | External Path \n"))

			i.pool.ClientPoolForeach(func(cl client.YproxyClient) error {
				_, err := clConn.Write([]byte(fmt.Sprintf("%v | %v | %v\n", cl.ID(), cl.OPType(), cl.ExternalFilePath())))

				return err
			})

			clConn.Close()
		}
	}()

	listener, err := net.Listen("unix", instanceCnf.SocketPath)
	if err != nil {
		ylogger.Zero.Error().Err(err).Msg("failed to start socket listener")
		return err
	}
	defer listener.Close()

	s := storage.NewStorage(
		&instanceCnf.StorageCnf,
	)

	var cr crypt.Crypter = nil
	if instanceCnf.CryptoCnf.GPGKeyPath != "" {
		cr, err = crypt.NewCrypto(&instanceCnf.CryptoCnf)
	}
	
	if err != nil {
		return err
	}

	notifier, err := sdnotifier.NewNotifier(instanceCnf.GetSystemdSocketPath(), instanceCnf.SystemdNotificationsDebug)
	if err != nil {
		ylogger.Zero.Error().Err(err).Msg("failed to initialize systemd notifier")
		if instanceCnf.SystemdNotificationsDebug {
			return err
		}
	}
	notifier.Ready()

	go func() {
		for {
			notifier.Notify()
			time.Sleep(sdnotifier.Timeout)
		}
	}()

	go func() {
		<-ctx.Done()
		os.Exit(0)
	}()

	for {
		clConn, err := listener.Accept()
		if err != nil {
			ylogger.Zero.Error().Err(err).Msg("failed to accept connection")
		}
		ylogger.Zero.Debug().Str("addr", clConn.LocalAddr().String()).Msg("accepted client connection")
		go func() {
			ycl := client.NewYClient(clConn)
			i.pool.Put(ycl)
			if err := proc.ProcConn(s, cr, ycl); err != nil {
				ylogger.Zero.Debug().Uint("id", ycl.ID()).Err(err).Msg("got error serving client")
			}
			_, err := i.pool.Pop(ycl.ID())
			if err != nil {
				// ?? wtf
				ylogger.Zero.Error().Uint("id", ycl.ID()).Err(err).Msg("got error erasing client from pool")
			}
		}()
	}
}
