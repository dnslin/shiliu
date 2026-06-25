package service

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "shiliu/api/v1"
)

func TestResolvePublicHostIPsRejectsHostnameResolvingToPrivateIP(t *testing.T) {
	_, err := resolvePublicHostIPs(context.Background(), staticResolver{
		ips: []net.IPAddr{{IP: net.ParseIP("169.254.169.254")}},
	}, "feed.attacker.example")

	assert.ErrorIs(t, err, v1.ErrFeedInvalidURL)
}

func TestResolvePublicHostIPsPropagatesResolverFailure(t *testing.T) {
	resolverErr := errors.New("resolver unavailable")
	_, err := resolvePublicHostIPs(context.Background(), staticResolver{err: resolverErr}, "feed.example")

	assert.ErrorIs(t, err, resolverErr)
}

type staticResolver struct {
	ips []net.IPAddr
	err error
}

func (r staticResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	return r.ips, r.err
}
