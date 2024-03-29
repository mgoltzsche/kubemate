package cmds

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	genericapiserver "k8s.io/apiserver/pkg/server"

	"github.com/mgoltzsche/kubemate/pkg/apiserver"
)

type ConnectConfig struct {
	apiserver.ServerOptions
	HTTPAddress     string
	AdvertiseIfaces []string
	LogLevel        string
}

var appName = "kubemate"
var Connect = ConnectConfig{
	ServerOptions: apiserver.NewServerOptions(),
}
var shutdownFile string
var ConnectFlags = []cli.Flag{
	cli.StringFlag{
		Name:        "http-address",
		Usage:       "(agent/runtime) net/IP to listen on without TLS",
		EnvVar:      "KUBEMATE_INSECURE_ADDRESS",
		Destination: &Connect.HTTPAddress,
		Value:       Connect.HTTPAddress,
	},
	cli.IntFlag{
		Name:        "http-port",
		Usage:       "(agent/runtime) non-TLS port to listen on.",
		EnvVar:      "KUBEMATE_INSECURE_PORT",
		Destination: &Connect.HTTPPort,
		Value:       Connect.HTTPPort,
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
	cli.StringSliceFlag{
		Name:   "advertise-iface",
		Usage:  "(agent/runtime) Name(s) of the network interface(s) to advertise via mdns",
		EnvVar: "KUBEMATE_ADVERTISE_IFACE",
		Value:  (*cli.StringSlice)(&Connect.AdvertiseIfaces),
	},
	cli.StringFlag{
		Name:        "web-dir",
		Usage:       "(agent/runtime) directory that holds the static web application",
		EnvVar:      "KUBEMATE_WEB_DIR",
		Destination: &Connect.WebDir,
		Value:       Connect.WebDir,
	},
	cli.StringFlag{
		Name:        "manifest-dir",
		Usage:       "(agent/runtime) directory that holds additional manifests the server should be initialized with",
		EnvVar:      "KUBEMATE_MANIFEST_DIR",
		Destination: &Connect.ManifestDir,
		Value:       Connect.ManifestDir,
	},
	cli.StringFlag{
		Name:        "data-dir",
		Usage:       "(agent/runtime) directory that holds the apiserver state",
		EnvVar:      "KUBEMATE_DATA_DIR",
		Destination: &Connect.DataDir,
		Value:       Connect.DataDir,
	},
	cli.StringSliceFlag{
		Name:   "kubelet-arg",
		Usage:  "(agent/flags) Customized flag for kubelet process",
		EnvVar: "KUBEMATE_KUBELET_ARG",
		Value:  (*cli.StringSlice)(&Connect.KubeletArgs),
	},
	cli.BoolFlag{
		Name:        "docker",
		Usage:       "(agent/runtime) enable docker support",
		EnvVar:      "KUBEMATE_DOCKER",
		Destination: &Connect.Docker,
	},
	cli.BoolFlag{
		Name:        "write-host-resolvconf",
		Usage:       "(agent/runtime) let kubemate copy /etc/resolv.conf to /host/etc/resolv.conf",
		EnvVar:      "KUBEMATE_WRITE_HOST_RESOLVCONF",
		Destination: &Connect.WriteHostResolvConf,
	},
	cli.StringFlag{
		Name:        "shutdown-file",
		Usage:       "(agent/runtime) write a file when a shutdown is initiated via the API",
		EnvVar:      "KUBEMATE_SHUTDOWN_FILE",
		Destination: &shutdownFile,
	},
	cli.StringFlag{
		Name:        "log-level",
		Usage:       "(agent/runtime) log level",
		EnvVar:      "KUBEMATE_LOG_LEVEL",
		Destination: &Connect.LogLevel,
		Value:       Connect.LogLevel,
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
	return run(genericapiserver.SetupSignalContext())
}

func run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	Connect.ServerOptions.Shutdown = func() error {
		if shutdownFile != "" {
			err := writeShutdownFile()
			if err != nil {
				return err
			}
		}
		cancel()
		return nil
	}
	genericServer, err := apiserver.NewServer(Connect.ServerOptions)
	if err != nil {
		return err
	}
	if Connect.LogLevel != "" {
		lvl, err := logrus.ParseLevel(Connect.LogLevel)
		if err != nil {
			return fmt.Errorf("unsupported --log-level %q specified", lvl)
		}
		logrus.SetLevel(lvl)
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
			if err != nil && err != http.ErrServerClosed {
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
	err = parallelize(ctx, daemons...)
	if err != nil {
		return err
	}
	return nil
}

func writeShutdownFile() error {
	err := os.WriteFile(shutdownFile, []byte{}, 0644)
	if err != nil {
		return fmt.Errorf("write shutdown file: %w", err)
	}
	return nil
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
