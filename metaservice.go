package prefab

import "context"

// Implements MetaServiceServer.
type meta struct {
	UnimplementedMetaServiceServer
	configs        map[string]string
	csrfSigningKey []byte
}

func (s *meta) ClientConfig(ctx context.Context, in *ClientConfigRequest) (*ClientConfigResponse, error) {
	return &ClientConfigResponse{
		CsrfToken: SendCSRFToken(ctx, s.csrfSigningKey),
		Configs:   s.configs,
	}, nil
}
