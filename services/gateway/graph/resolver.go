package graph

import (
    "errors"

    "google.golang.org/grpc/status"

    productsv1 "github.com/Pali912/ecommerce-platform/gen/products/v1"
    usersv1 "github.com/Pali912/ecommerce-platform/gen/users/v1"
    "github.com/Pali912/ecommerce-platform/services/gateway/graph/model"
)

// Resolver holds the gRPC clients the resolvers use to reach the other services.
type Resolver struct {
    UsersClient    usersv1.UsersServiceClient
    ProductsClient productsv1.ProductsServiceClient
}

// gqlError turns a gRPC error into a clean message for GraphQL clients.
func gqlError(err error) error {
    if s, ok := status.FromError(err); ok {
        return errors.New(s.Message())
    }
    return err
}

func userFromProto(u *usersv1.User) *model.User {
    return &model.User{ID: u.GetId(), Email: u.GetEmail(), Name: u.GetName()}
}

func productFromProto(p *productsv1.Product) *model.Product {
    return &model.Product{
        ID:          p.GetId(),
        Name:        p.GetName(),
        Description: p.GetDescription(),
        PriceCents:  int32(p.GetPriceCents()),
        Currency:    p.GetCurrency(),
    }
}

