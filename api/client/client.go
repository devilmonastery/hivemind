// Package client provides a small, reusable gRPC client for external consumers
// of the Hivemind backend (for example, other bots in the same cluster).
//
// It authenticates with a static service-account token and exposes typed
// service accessors. By default the connection is plaintext (h2c), which is
// appropriate for in-cluster use where the Hivemind server listens without TLS
// (hivemind-server.<namespace>:4153). Use WithTLS for out-of-cluster callers.
package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"

	quotespb "github.com/devilmonastery/hivemind/api/generated/go/quotespb"
	wikipb "github.com/devilmonastery/hivemind/api/generated/go/wikipb"
)

const (
	mdAuthorization  = "authorization"
	mdDiscordUserID  = "x-discord-user-id"
	mdDiscordGuildID = "x-discord-guild-id"
)

// Client wraps a gRPC connection to the Hivemind server and exposes typed
// service clients. It is safe for concurrent use.
type Client struct {
	conn   *grpc.ClientConn
	quotes quotespb.QuoteServiceClient
	wiki   wikipb.WikiServiceClient
}

type options struct {
	tlsConfig *tls.Config
	dialOpts  []grpc.DialOption
}

// Option configures a Client.
type Option func(*options)

// WithTLS enables TLS using the given SNI server name. Use this for callers
// outside the cluster; in-cluster callers should use the default plaintext mode.
func WithTLS(serverName string) Option {
	return func(o *options) {
		o.tlsConfig = &tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12}
	}
}

// WithDialOption appends a raw grpc.DialOption (advanced use).
func WithDialOption(opt grpc.DialOption) Option {
	return func(o *options) { o.dialOpts = append(o.dialOpts, opt) }
}

// New creates a Hivemind client for the given address (host:port), authenticated
// with the provided service-account token. The token is attached as an
// "authorization: Bearer <token>" header on every RPC.
func New(addr, token string, opts ...Option) (*Client, error) {
	if addr == "" {
		return nil, fmt.Errorf("hivemind client: address is required")
	}
	if token == "" {
		return nil, fmt.Errorf("hivemind client: service token is required")
	}

	var o options
	for _, fn := range opts {
		fn(&o)
	}

	dialOpts := []grpc.DialOption{
		grpc.WithUnaryInterceptor(staticTokenUnaryInterceptor(token)),
		grpc.WithStreamInterceptor(staticTokenStreamInterceptor(token)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             3 * time.Second,
			PermitWithoutStream: true,
		}),
	}
	if o.tlsConfig != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(o.tlsConfig)))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	dialOpts = append(dialOpts, o.dialOpts...)

	conn, err := grpc.NewClient(addr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("hivemind client: failed to dial %q: %w", addr, err)
	}

	return &Client{
		conn:   conn,
		quotes: quotespb.NewQuoteServiceClient(conn),
		wiki:   wikipb.NewWikiServiceClient(conn),
	}, nil
}

// Quotes returns the QuoteService client.
func (c *Client) Quotes() quotespb.QuoteServiceClient { return c.quotes }

// Wiki returns the WikiService client.
func (c *Client) Wiki() wikipb.WikiServiceClient { return c.wiki }

// Conn returns the underlying gRPC connection.
func (c *Client) Conn() *grpc.ClientConn { return c.conn }

// Close closes the underlying gRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// WithDiscordContext attaches optional Discord identity metadata to the outgoing
// context. Pass an empty userID to omit it, which keeps a request scoped only by
// its explicit guild_id and avoids per-user membership ACL filtering on the server.
func WithDiscordContext(ctx context.Context, guildID, userID string) context.Context {
	if guildID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, mdDiscordGuildID, guildID)
	}
	if userID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, mdDiscordUserID, userID)
	}
	return ctx
}

func staticTokenUnaryInterceptor(token string) grpc.UnaryClientInterceptor {
	bearer := "Bearer " + token
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, mdAuthorization, bearer)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func staticTokenStreamInterceptor(token string) grpc.StreamClientInterceptor {
	bearer := "Bearer " + token
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = metadata.AppendToOutgoingContext(ctx, mdAuthorization, bearer)
		return streamer(ctx, desc, cc, method, opts...)
	}
}
