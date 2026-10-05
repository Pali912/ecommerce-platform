package server

import (
    "context"
    "errors"

    "github.com/google/uuid"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/protobuf/types/known/timestamppb"

    productsv1 "github.com/Pali912/ecommerce-platform/gen/products/v1"
    "github.com/Pali912/ecommerce-platform/services/products/internal/repo"
)

type Server struct {
    productsv1.UnimplementedProductsServiceServer
    repo *repo.Repo
}

func New(r *repo.Repo) *Server {
    return &Server{repo: r}
}

func toProto(p *repo.Product) *productsv1.Product {
    return &productsv1.Product{
        Id:          p.ID,
        Name:        p.Name,
        Description: p.Description,
        PriceCents:  p.PriceCents,
        Currency:    p.Currency,
        CreatedAt:   timestamppb.New(p.CreatedAt),
    }
}

func (s *Server) CreateProduct(ctx context.Context, req *productsv1.CreateProductRequest) (*productsv1.CreateProductResponse, error) {
    if req.GetName() == "" {
        return nil, status.Error(codes.InvalidArgument, "name is required")
    }
    if req.GetPriceCents() < 0 {
        return nil, status.Error(codes.InvalidArgument, "price must not be negative")
    }
    currency := req.GetCurrency()
    if currency == "" {
        currency = "USD"
    }

    p, err := s.repo.Create(ctx, req.GetName(), req.GetDescription(), req.GetPriceCents(), currency)
    if err != nil {
        return nil, status.Error(codes.Internal, "could not create product")
    }
    return &productsv1.CreateProductResponse{Product: toProto(p)}, nil
}

func (s *Server) GetProduct(ctx context.Context, req *productsv1.GetProductRequest) (*productsv1.GetProductResponse, error) {
    if _, err := uuid.Parse(req.GetId()); err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid product id")
    }
    p, err := s.repo.GetByID(ctx, req.GetId())
    if errors.Is(err, repo.ErrNotFound) {
        return nil, status.Error(codes.NotFound, "product not found")
    }
    if err != nil {
        return nil, status.Error(codes.Internal, "could not get product")
    }
    return &productsv1.GetProductResponse{Product: toProto(p)}, nil
}

func (s *Server) GetProductsByIDs(ctx context.Context, req *productsv1.GetProductsByIDsRequest) (*productsv1.GetProductsByIDsResponse, error) {
    for _, id := range req.GetIds() {
        if _, err := uuid.Parse(id); err != nil {
            return nil, status.Error(codes.InvalidArgument, "invalid product id: "+id)
        }
    }
    list, err := s.repo.GetByIDs(ctx, req.GetIds())
    if err != nil {
        return nil, status.Error(codes.Internal, "could not get products")
    }
    out := make([]*productsv1.Product, 0, len(list))
    for _, p := range list {
        out = append(out, toProto(p))
    }
    return &productsv1.GetProductsByIDsResponse{Products: out}, nil
}

func (s *Server) ListProducts(ctx context.Context, req *productsv1.ListProductsRequest) (*productsv1.ListProductsResponse, error) {
    limit := req.GetLimit()
    if limit <= 0 {
        limit = 20
    }
    if limit > 100 {
        limit = 100
    }
    offset := req.GetOffset()
    if offset < 0 {
        offset = 0
    }

    list, err := s.repo.List(ctx, limit, offset)
    if err != nil {
        return nil, status.Error(codes.Internal, "could not list products")
    }
    out := make([]*productsv1.Product, 0, len(list))
    for _, p := range list {
        out = append(out, toProto(p))
    }
    return &productsv1.ListProductsResponse{Products: out}, nil
}
