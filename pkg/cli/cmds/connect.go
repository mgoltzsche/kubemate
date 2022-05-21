package cmds

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mgoltzsche/k3spi/pkg/apiserver"
	"github.com/mgoltzsche/k3spi/pkg/runner"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

type ConnectConfig struct {
	Address string
	WebDir  string
	Docker  bool
}

var appName = "k3s-connect"
var Connect ConnectConfig
var ConnectFlags = []cli.Flag{
	cli.StringFlag{
		Name:        "address",
		Usage:       "(agent/runtime) enable docker support",
		EnvVar:      "K3SCONNECT_ADDRESS",
		Destination: &Connect.Address,
	},
	cli.StringFlag{
		Name:        "web-dir",
		Usage:       "(agent/runtime) enable docker support",
		EnvVar:      "K3SCONNECT_WEB_DIR",
		Destination: &Connect.WebDir,
	},
	cli.BoolFlag{
		Name:        "docker",
		Usage:       "(agent/runtime) enable docker support",
		EnvVar:      "K3SCONNECT_DOCKER",
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
	args := []string{
		"server",
		"--disable-cloud-controller",
		"--disable-helm-controller",
		"--no-deploy=servicelb,traefik,metrics-server",
	}
	if Connect.Docker {
		args = append(args, "--docker")
	}
	/*// TODO: replace the code below with the new runner.
	// TODO: using the generic apiserver, try to implement a DelegationTarget to proxy to k3s? see https://github.com/kubernetes/apiserver/blob/7816c29325f8e9272c1155bc82d4a25fe09bb683/pkg/server/genericapiserver.go#L343
	c := exec.CommandContext(ctx, "/proc/self/exe", args...)
	c.Env = os.Environ()
	stdout, err := c.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdout.Close()
	stderr, err := c.StderrPipe()
	if err != nil {
		return err
	}
	defer stderr.Close()
	err = c.Start()
	if err != nil {
		return err
	}
	go func() { _, _ = io.Copy(os.Stdout, stdout) }()
	go func() { _, _ = io.Copy(os.Stderr, stderr) }()
	return c.Wait()*/

	daemon := runner.NewRunner()
	ch := daemon.Start(context.Background())
	go func() {
		for cmd := range ch {
			// TODO: pass back status changes to the frontend
			logrus.Printf("k3s %s: %s", cmd.Status.State, cmd.Status.Message)
		}
	}()

	opts := apiserver.NewServerOptions()
	opts.WebDir = Connect.WebDir
	opts.Address = Connect.Address
	genericServer, err := apiserver.NewServer(opts)
	if err != nil {
		return err
	}
	genericServer.AddPostStartHookOrDie("k3s-connect", func(ctx genericapiserver.PostStartHookContext) error {
		daemon.SetCommand(runner.CommandSpec{
			Command: "/proc/self/exe",
			Args:    args,
		})
		return nil
	})
	genericServer.AddPreShutdownHookOrDie("k3s-connect", func() error {
		return daemon.Close()
	})
	prepared := genericServer.PrepareRun()
	srv := &http.Server{
		Addr: opts.Address,
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
