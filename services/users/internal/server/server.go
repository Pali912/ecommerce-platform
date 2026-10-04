package server

import (
	"context"
	"errors"
	"net/mail"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	usersv1 "github.com/Pali912/ecommerce-platform/gen/users/v1"
	"github.com/Pali912/ecommerce-platform/services/users/internal/auth"
	"github.com/Pali912/ecommerce-platform/services/users/internal/repo"
)

type Server struct {
	usersv1.UnimplementedUsersServiceServer
	repo *repo.Repo
	auth *auth.Manager
}

func New(r *repo.Repo, a *auth.Manager) *Server {
	return &Server{repo: r, auth: a}
}

func toProto(u *repo.User) *usersv1.User {
	return &usersv1.User{
		Id:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		CreatedAt: timestamppb.New(u.CreatedAt),
	}
}

func (s *Server) Register(ctx context.Context, req *usersv1.RegisterRequest) (*usersv1.RegisterResponse, error) {
	if _, err := mail.ParseAddress(req.GetEmail()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid email")
	}
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if len(req.GetPassword()) < 8 {
		return nil, status.Error(codes.InvalidArgument, "password must be at least 8 characters")
	}

	hash, err := auth.HashPassword(req.GetPassword())
	if err != nil {
		return nil, status.Error(codes.Internal, "could not process password")
	}

	u, err := s.repo.Create(ctx, req.GetEmail(), req.GetName(), hash)
	if errors.Is(err, repo.ErrEmailTaken) {
		return nil, status.Error(codes.AlreadyExists, "email already registered")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "could not create user")
	}

	token, err := s.auth.Issue(u.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not issue token")
	}
	return &usersv1.RegisterResponse{Token: token, User: toProto(u)}, nil
}

func (s *Server) Login(ctx context.Context, req *usersv1.LoginRequest) (*usersv1.LoginResponse, error) {
	u, err := s.repo.GetByEmail(ctx, req.GetEmail())
	if errors.Is(err, repo.ErrNotFound) {
		return nil, status.Error(codes.Unauthenticated, "invalid email or password")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "login failed")
	}
	if !auth.CheckPassword(u.PasswordHash, req.GetPassword()) {
		return nil, status.Error(codes.Unauthenticated, "invalid email or password")
	}

	token, err := s.auth.Issue(u.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not issue token")
	}
	return &usersv1.LoginResponse{Token: token, User: toProto(u)}, nil
}

func (s *Server) GetUser(ctx context.Context, req *usersv1.GetUserRequest) (*usersv1.GetUserResponse, error) {
	if _, err := uuid.Parse(req.GetId()); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user id")
	}
	u, err := s.repo.GetByID(ctx, req.GetId())
	if errors.Is(err, repo.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "could not get user")
	}
	return &usersv1.GetUserResponse{User: toProto(u)}, nil
}

func (s *Server) GetUsersByIDs(ctx context.Context, req *usersv1.GetUsersByIDsRequest) (*usersv1.GetUsersByIDsResponse, error) {
	for _, id := range req.GetIds() {
		if _, err := uuid.Parse(id); err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid user id: "+id)
		}
	}
	users, err := s.repo.GetByIDs(ctx, req.GetIds())
	if err != nil {
		return nil, status.Error(codes.Internal, "could not get users")
	}
	out := make([]*usersv1.User, 0, len(users))
	for _, u := range users {
		out = append(out, toProto(u))
	}
	return &usersv1.GetUsersByIDsResponse{Users: out}, nil
}

func (s *Server) ValidateToken(ctx context.Context, req *usersv1.ValidateTokenRequest) (*usersv1.ValidateTokenResponse, error) {
	id, err := s.auth.Verify(req.GetToken())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid token")
	}
	return &usersv1.ValidateTokenResponse{UserId: id}, nil
}