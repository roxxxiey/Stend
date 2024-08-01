package g_app

import (
	grpc_client "ForStend/gRPC"
	"ForStend/internal/devConf"
	"context"
	"flag"
	"fmt"
	sh "github.com/roxxxiey/ProtoForStend/go"
	"google.golang.org/grpc"
	"log/slog"
	"net"
)

type App struct {
	log        *slog.Logger
	gRPCServer *grpc.Server
	port       int
}

// New create new gRPC server app
func New(
	log *slog.Logger,
	port int,
) *App {
	gRPCServer := grpc.NewServer()

	grpc_client.RegisterSSHClient(gRPCServer)

	return &App{
		log:        log,
		gRPCServer: gRPCServer,
		port:       port,
	}
}

func (a *App) MustRun() {
	if err := a.RunGRPCServer(); err != nil {
		panic(err)
	}

}

func (a *App) RunGRPCServer() error {
	const msg = "gRPCApp.RUN"

	log := a.log.With(
		slog.String("msg", msg),
		slog.Int("port", a.port),
	)

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", a.port))

	if err != nil {
		return fmt.Errorf("%s: %w", msg, err)
	}

	log.Info("GRPC Server is running", slog.String("addr", l.Addr().String()))

	ctx := context.Background()

	getter := flag.Lookup("pathfile")
	pathFile := getter.Value.String()

	cfg2 := devConf.InitConf()

	ips := cfg2.IP

	var settings []*sh.Settings
	settings = append(settings, &sh.Settings{
		Name:  "Device Ip",
		Value: "",
	})
	settings = append(settings, &sh.Settings{
		Name:  "Username",
		Value: cfg2.StendData.Username,
	})
	settings = append(settings, &sh.Settings{
		Name:  "Password",
		Value: cfg2.StendData.Password,
	})
	settings = append(settings, &sh.Settings{
		Name:  "Path to File",
		Value: pathFile,
	})
	settings = append(settings, &sh.Settings{
		Name:  "TftpServerIp",
		Value: cfg2.StendData.TftpServerIp,
	})

	var good []string
	var problems []string
	count := 0
	for _, ip := range ips {
		settings[0].Value = ip
		resp := &sh.UpdateFirmwareRequest{
			Settings: settings,
		}
		fmt.Println(resp)
		client := grpc_client.TFTPviaSSH{}
		result, err := client.UpdateFirmware(ctx, resp)
		if err != nil {
			problems = append(problems, settings[0].Value)
		} else {
			count++
			good = append(good, settings[0].Value)
		}
		fmt.Println()
		fmt.Printf("The result of firmware the device from %v: %v", settings[0].Value, result)
	}

	fmt.Println()
	fmt.Printf("%v devices have been firmware: %v", count, good)
	fmt.Println()
	fmt.Println("The IP of the devices that received the error: ", problems)

	if err := a.gRPCServer.Serve(l); err != nil {
		return fmt.Errorf("%s: %w", msg, err)
	}

	return nil
}

// Stop gRPC server
func (a *App) Stop() {
	const msg = "gRPCApp.STOP"

	a.log.With(slog.String("msg", msg)).
		Info("GRPC Server is stopping", slog.Int("port", a.port))

	a.gRPCServer.GracefulStop()
}
