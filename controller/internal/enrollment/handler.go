package enrollment

// Handler will host gRPC adapters for Enroll and Renew once protobuf stubs exist.
type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}
