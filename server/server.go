package server

import "integratorV2/config"

type Server struct {
	config *config.Config
	db     interface{}
}

func NewServer(cfg *config.Config, db interface{}) *Server {
	return &Server{config: cfg, db: db}
}

func (s *Server) Start(addr string) error {
	return nil
}
