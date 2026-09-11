package billingclient

import (
	"context"

	billingv1 "github.com/Bremcm/uptime/internal/pb/billing/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Limits struct {
	PlanName           string
	MaxMonitors        int
	MinIntervalSeconds int
	RateLimitPerMinute int
}

type Client struct {
	grpcClient billingv1.BillingServiceClient
}

func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{grpcClient: billingv1.NewBillingServiceClient(conn)}, nil
}

func (c *Client) GetLimits(ctx context.Context, userID int64) (Limits, error) {
	resp, err := c.grpcClient.GetLimits(ctx, &billingv1.GetLimitsRequest{UserId: userID})
	if err != nil {
		return Limits{}, err
	}
	return Limits{
		PlanName:           resp.PlanName,
		MaxMonitors:        int(resp.MaxMonitors),
		MinIntervalSeconds: int(resp.MinIntervalSeconds),
		RateLimitPerMinute: int(resp.RateLimitPerMinute),
	}, nil
}
