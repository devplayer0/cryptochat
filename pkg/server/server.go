package server

// Server is a CryptoChat server
type Server struct {
}

// NewServer creates a new Server
func NewServer() *Server {
	return &Server{}
}

// Start begins listening
func (s *Server) Start() error {
	return nil
}

// Stop ends listening
func (s *Server) Stop() {

}
