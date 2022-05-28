package cmds

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mgoltzsche/kubemate/pkg/apiserver"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type ConnectConfig struct {
	apiserver.ServerOptions
	HTTPAddress string
	HTTPPort    int
}

var appName = "kubemate"
var Connect = ConnectConfig{
	ServerOptions: apiserver.NewServerOptions(),
}
var ConnectFlags = []cli.Flag{
	cli.StringFlag{
		Name:        "http-address",
		Usage:       "(agent/runtime) net/IP to listen on without TLS",
		EnvVar:      "KUBEMATE_INSECURE_ADDRESS",
		Destination: &Connect.HTTPAddress,
		Value:       "127.0.0.1",
	},
	cli.IntFlag{
		Name:        "http-port",
		Usage:       "(agent/runtime) non-TLS port to listen on",
		EnvVar:      "KUBEMATE_INSECURE_PORT",
		Destination: &Connect.HTTPPort,
		Value:       8080,
	},
	cli.StringFlag{
		Name:        "https-address",
		Usage:       "(agent/runtime) net/IP to listen on with TLS",
		EnvVar:      "KUBEMATE_SECURE_ADDRESS",
		Destination: &Connect.HTTPSAddress,
		Value:       Connect.HTTPSAddress,
	},
	cli.IntFlag{
		Name:        "https-port",
		Usage:       "(agent/runtime) TLS port to listen on",
		EnvVar:      "KUBEMATE_SECURE_PORT",
		Destination: &Connect.HTTPSPort,
		Value:       Connect.HTTPSPort,
	},
	cli.StringFlag{
		Name:        "web-dir",
		Usage:       "(agent/runtime) enable docker support",
		EnvVar:      "KUBEMATE_WEB_DIR",
		Destination: &Connect.WebDir,
		Value:       Connect.WebDir,
	},
	cli.BoolFlag{
		Name:        "docker",
		Usage:       "(agent/runtime) enable docker support",
		EnvVar:      "KUBEMATE_DOCKER",
		Destination: &Connect.Docker,
	},
}

func NewConnectCommand(action func(*cli.Context) error) cli.Command {
	return cli.Command{
		Name:      "connect",
		Usage:     "Run API and UI to create or join a cluster",
		UsageText: appName + " connect [OPTIONS]",
		Action:    action,
		Flags:     ConnectFlags,
	}
}

func RunConnectServer(app *cli.Context) error {
	return run(newContext())
}

func run(ctx context.Context) error {
	genericServer, err := apiserver.NewServer(Connect.ServerOptions)
	if err != nil {
		return err
	}
	prepared := genericServer.PrepareRun()
	srv := &http.Server{
		Addr: fmt.Sprintf("%s:%d", Connect.HTTPAddress, Connect.HTTPPort),
	}
	srv.Handler = prepared.Handler
	daemons := []func(context.Context) error{
		func(ctx context.Context) error {
			go func() {
				<-ctx.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := srv.Shutdown(ctx); err != nil {
					logrus.Println("error: failed to shut down server:", err)
				}
				cancel()
			}()
			err := srv.ListenAndServe()
			if err != nil {
				return fmt.Errorf("http server: %w", err)
			}
			return nil
		},
		func(ctx context.Context) error {
			err := prepared.Run(ctx.Done())
			if err != nil {
				return fmt.Errorf("api server: %w", err)
			}
			return nil
		},
	}
	return parallelize(ctx, daemons...)
}

func newContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1) // exit immediately on 2nd signal
	}()
	return ctx
}

// parallelize runs the provided methods concurrently and cancels the context when any of them returns.
func parallelize(ctx context.Context, daemons ...func(context.Context) error) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan error, len(daemons))
	for _, fn := range daemons {
		go func(fn func(context.Context) error) {
			err := fn(ctx)
			done <- err
			cancel()
		}(fn)
	}
	for i := 0; i < len(daemons); i++ {
		e := <-done
		if err == nil {
			err = e
		}
	}
	cancel()
	close(done)
	return err
}
