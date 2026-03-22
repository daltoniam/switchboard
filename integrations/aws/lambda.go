package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	mcp "github.com/daltoniam/switchboard"
)

func lambdaListFunctions(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &lambda.ListFunctionsInput{}
	if v := r.Int32("max_items"); v > 0 {
		input.MaxItems = aws.Int32(v)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.lambdaClient.ListFunctions(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func lambdaGetFunction(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fnName := r.Str("function_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.lambdaClient.GetFunction(ctx, &lambda.GetFunctionInput{
		FunctionName: aws.String(fnName),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func lambdaInvoke(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &lambda.InvokeInput{
		FunctionName: aws.String(r.Str("function_name")),
	}
	if payload := r.Str("payload"); payload != "" {
		input.Payload = []byte(payload)
	}
	if invType := r.Str("invocation_type"); invType != "" {
		input.InvocationType = lambdatypes.InvocationType(invType)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.lambdaClient.Invoke(ctx, input)
	if err != nil {
		return errResult(err)
	}

	type invokeResult struct {
		StatusCode    int32  `json:"status_code"`
		FunctionError string `json:"function_error,omitempty"`
		Payload       string `json:"payload"`
	}
	result := invokeResult{
		StatusCode: out.StatusCode,
		Payload:    string(out.Payload),
	}
	if out.FunctionError != nil {
		result.FunctionError = *out.FunctionError
	}
	return mcp.JSONResult(result)
}

func lambdaListEventSourceMappings(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	input := &lambda.ListEventSourceMappingsInput{}
	if fn := r.Str("function_name"); fn != "" {
		input.FunctionName = aws.String(fn)
	}
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.lambdaClient.ListEventSourceMappings(ctx, input)
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}

func lambdaGetFunctionConfiguration(ctx context.Context, a *integration, args map[string]any) (*mcp.ToolResult, error) {
	r := mcp.NewArgs(args)
	fnName := r.Str("function_name")
	if err := r.Err(); err != nil {
		return mcp.ErrResult(err)
	}
	out, err := a.lambdaClient.GetFunctionConfiguration(ctx, &lambda.GetFunctionConfigurationInput{
		FunctionName: aws.String(fnName),
	})
	if err != nil {
		return errResult(err)
	}
	return mcp.JSONResult(out)
}
