package server

import (
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_tags "github.com/grpc-ecosystem/go-grpc-middleware/tags"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
)

var grpcServer *grpc.Server

func Start(lisAddr, storePath string, cb func() []byte) {
	logrusEntry := getLogrusEntry()

	lis, err := net.Listen("tcp", lisAddr)
	if err != nil {
		log.Fatalf("failed to listen %v", err)
	}

	grpcServer = grpc.NewServer(
		grpc_middleware.WithUnaryServerChain(
			grpc_tags.UnaryServerInterceptor(
				grpc_tags.WithFieldExtractor(
					grpc_tags.CodeGenRequestFieldExtractor,
				),
			),
			grpc_logrus.UnaryServerInterceptor(logrusEntry),
			grpc_recovery.UnaryServerInterceptor(),
		),
	)

	RegisterSecretFeederSvcServer(grpcServer, NewSecretFeederHandler(storePath, cb, logrusEntry))

	logrusEntry.Infoln("Listen address", lis.Addr().String(), "Store", storePath)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func Stop() {
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}
	grpcServer = nil
}

func getLogrusEntry() *logrus.Entry {
	//logrus.SetFormatter(&logrus.TextFormatter{})
	logrus.SetFormatter(&logrus.JSONFormatter{PrettyPrint: true, DisableTimestamp: false})
	logrus.SetReportCaller(false)
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.TraceLevel)
	//logrus.ErrorKey = "khole.error"
	//logrusEntry := logrus.WithFields(logrus.Fields{"Fields": "GRPC", "Module": "Root"})
	return logrus.NewEntry(logrus.StandardLogger())
}
