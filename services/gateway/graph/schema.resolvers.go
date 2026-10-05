package graph

import (
    "context"

    productsv1 "github.com/Pali912/ecommerce-platform/gen/products/v1"
    usersv1 "github.com/Pali912/ecommerce-platform/gen/users/v1"
    "github.com/Pali912/ecommerce-platform/services/gateway/graph/model"
)

// Register is the resolver for the register field.
func (r *mutationResolver) Register(ctx context.Context, email string, password string, name string) (*model.AuthPayload, error) {
    res, err := r.UsersClient.Register(ctx, &usersv1.RegisterRequest{Email: email, Password: password, Name: name})
    if err != nil {
        return nil, gqlError(err)
    }
    return &model.AuthPayload{Token: res.GetToken(), User: userFromProto(res.GetUser())}, nil
}

// Login is the resolver for the login field.
func (r *mutationResolver) Login(ctx context.Context, email string, password string) (*model.AuthPayload, error) {
    res, err := r.UsersClient.Login(ctx, &usersv1.LoginRequest{Email: email, Password: password})
    if err != nil {
        return nil, gqlError(err)
    }
    return &model.AuthPayload{Token: res.GetToken(), User: userFromProto(res.GetUser())}, nil
}

// CreateProduct is the resolver for the createProduct field.
func (r *mutationResolver) CreateProduct(ctx context.Context, name string, description *string, priceCents int32) (*model.Product, error) {
    desc := ""
    if description != nil {
        desc = *description
    }
    res, err := r.ProductsClient.CreateProduct(ctx, &productsv1.CreateProductRequest{
        Name:        name,
        Description: desc,
        PriceCents:  int64(priceCents),
    })
    if err != nil {
        return nil, gqlError(err)
    }
    return productFromProto(res.GetProduct()), nil
}

// Product is the resolver for the product field.
func (r *queryResolver) Product(ctx context.Context, id string) (*model.Product, error) {
    res, err := r.ProductsClient.GetProduct(ctx, &productsv1.GetProductRequest{Id: id})
    if err != nil {
        return nil, gqlError(err)
    }
    return productFromProto(res.GetProduct()), nil
}

// Products is the resolver for the products field.
func (r *queryResolver) Products(ctx context.Context, limit *int32, offset *int32) ([]*model.Product, error) {
    req := &productsv1.ListProductsRequest{}
    if limit != nil {
        req.Limit = *limit
    }
    if offset != nil {
        req.Offset = *offset
    }
    res, err := r.ProductsClient.ListProducts(ctx, req)
    if err != nil {
        return nil, gqlError(err)
    }
    out := make([]*model.Product, 0, len(res.GetProducts()))
    for _, p := range res.GetProducts() {
        out = append(out, productFromProto(p))
    }
    return out, nil
}

// Mutation returns MutationResolver implementation.
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

// Query returns QueryResolver implementation.
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

type (
    mutationResolver struct{ *Resolver }
    queryResolver    struct{ *Resolver }
)

