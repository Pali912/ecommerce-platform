package server

import (
"context"
"testing"

"google.golang.org/grpc/codes"
"google.golang.org/grpc/status"

productsv1 "github.com/Pali912/ecommerce-platform/gen/products/v1"
)

func TestCreateProductValidation(t *testing.T) {
s := New(nil) // validation fails before the database is touched

tests := []struct {
name string
req  *productsv1.CreateProductRequest
}{
{"empty name", &productsv1.CreateProductRequest{Name: "", PriceCents: 100}},
{"negative price", &productsv1.CreateProductRequest{Name: "Mouse", PriceCents: -1}},
}

for _, tc := range tests {
t.Run(tc.name, func(t *testing.T) {
_, err := s.CreateProduct(context.Background(), tc.req)
if status.Code(err) != codes.InvalidArgument {
t.Errorf("got %v, want InvalidArgument", status.Code(err))
}
})
}
}

func TestGetProductInvalidID(t *testing.T) {
s := New(nil)
_, err := s.GetProduct(context.Background(), &productsv1.GetProductRequest{Id: "not-a-uuid"})
if status.Code(err) != codes.InvalidArgument {
t.Errorf("got %v, want InvalidArgument", status.Code(err))
}
}

func TestGetProductsByIDsInvalidID(t *testing.T) {
s := New(nil)
_, err := s.GetProductsByIDs(context.Background(), &productsv1.GetProductsByIDsRequest{Ids: []string{"bad"}})
if status.Code(err) != codes.InvalidArgument {
t.Errorf("got %v, want InvalidArgument", status.Code(err))
}
}
