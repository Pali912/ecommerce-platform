package main

import (
    "context"
    "log/slog"
    "net"
    "os"
    "os/signal"
    "syscall"

    "github.com/jackc/pgx/v5/pgxpool"
    "google.golang.org/grpc"
    "google.golang.org/grpc/reflection"

    productsv1 "github.com/Pali912/ecommerce-platform/gen/products/v1"
    "github.com/Pali912/ecommerce-platform/services/products/internal/repo"
    "github.com/Pali912/ecommerce-platform/services/products/internal/server"
)

func getenv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

func main() {
    log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    dbURL := getenv("PRODUCTS_DB_URL", "postgres://ecom:ecom@localhost:5432/products")
    addr := getenv("PRODUCTS_GRPC_ADDR", ":50052")

    pool, err := pgxpool.New(ctx, dbURL)
    if err != nil {
        log.Error("could not create db pool", "err", err)
        os.Exit(1)
    }
    defer pool.Close()

    if err := pool.Ping(ctx); err != nil {
        log.Error("could not reach database", "err", err)
        os.Exit(1)
    }

    lis, err := net.Listen("tcp", addr)
    if err != nil {
        log.Error("could not listen", "addr", addr, "err", err)
        os.Exit(1)
    }

    grpcServer := grpc.NewServer()
    productsv1.RegisterProductsServiceServer(grpcServer, server.New(repo.New(pool)))
    reflection.Register(grpcServer)

    go func() {
        <-ctx.Done()
        log.Info("shutting down")
        grpcServer.GracefulStop()
    }()

    log.Info("products service listening", "addr", addr)
    if err := grpcServer.Serve(lis); err != nil {
        log.Error("server stopped", "err", err)
        os.Exit(1)
    }
}
