package main

import (
    "context"
    "errors"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/99designs/gqlgen/graphql/handler"
    "github.com/99designs/gqlgen/graphql/handler/extension"
    "github.com/99designs/gqlgen/graphql/handler/transport"
    "github.com/99designs/gqlgen/graphql/playground"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    productsv1 "github.com/Pali912/ecommerce-platform/gen/products/v1"
    usersv1 "github.com/Pali912/ecommerce-platform/gen/users/v1"
    "github.com/Pali912/ecommerce-platform/services/gateway/graph"
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

    usersAddr := getenv("USERS_ADDR", "localhost:50051")
    productsAddr := getenv("PRODUCTS_ADDR", "localhost:50052")
    httpAddr := getenv("GATEWAY_HTTP_ADDR", ":8080")

    creds := grpc.WithTransportCredentials(insecure.NewCredentials())

    usersConn, err := grpc.NewClient(usersAddr, creds)
    if err != nil {
        log.Error("could not create users client", "err", err)
        os.Exit(1)
    }
    defer usersConn.Close()

    productsConn, err := grpc.NewClient(productsAddr, creds)
    if err != nil {
        log.Error("could not create products client", "err", err)
        os.Exit(1)
    }
    defer productsConn.Close()

    resolver := &graph.Resolver{
        UsersClient:    usersv1.NewUsersServiceClient(usersConn),
        ProductsClient: productsv1.NewProductsServiceClient(productsConn),
    }

    srv := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))
    srv.AddTransport(transport.Options{})
    srv.AddTransport(transport.GET{})
    srv.AddTransport(transport.POST{})
    srv.Use(extension.Introspection{})

    mux := http.NewServeMux()
    mux.Handle("/", playground.Handler("GraphQL playground", "/query"))
    mux.Handle("/query", srv)

    httpServer := &http.Server{
        Addr:              httpAddr,
        Handler:           mux,
        ReadHeaderTimeout: 5 * time.Second,
    }

    go func() {
        <-ctx.Done()
        log.Info("shutting down")
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _ = httpServer.Shutdown(shutdownCtx)
    }()

    log.Info("gateway listening", "addr", httpAddr, "users", usersAddr, "products", productsAddr)
    if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
        log.Error("server stopped", "err", err)
        os.Exit(1)
    }
}
