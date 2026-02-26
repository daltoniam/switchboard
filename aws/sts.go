package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sts"
	mcp "github.com/daltoniam/switchboard"
)

func getCallerIdentity(ctx context.Context, a *integration, _ map[string]any) (*mcp.ToolResult, error) {
	out, err := a.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return errResult(err)
	}
	return jsonResult(out)
}
